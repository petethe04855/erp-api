package catalog

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
)

// Create reproduces the workspace CreateProduct flow: forced non-bundle,
// accessory validation, then ONE transaction creating the SKU master row, the
// initial stock row and (when quantity > 0) the opening_stock movement plus
// bundle/accessory definitions.
func (u *usecase) Create(ctx context.Context, input CreateInput) (*ProductRecord, error) {
	skuCode := strings.ToUpper(strings.TrimSpace(input.SKU))
	if skuCode == "" {
		return nil, appErr("BAD_REQUEST", "SKU is required", 400)
	}
	if input.Name == "" {
		return nil, appErr("BAD_REQUEST", "Product name is required", 400)
	}

	// Spec: SKU ใหม่และการแก้ไข SKU ถูกบังคับเป็นสินค้าปกติ (is_bundle = false)
	input.IsBundle = false
	if len(input.Accessories) > 0 {
		if appErrA := u.validateAccessories(ctx, skuCode, input.Accessories); appErrA != nil {
			return nil, appErrA
		}
	}

	var createdAccessories []SKUAccessoryRecord
	var skuEntity domainSKU.SKU
	err := u.txMgr.Transaction(ctx, func(txCtx context.Context) error {
		skuEntity = domainSKU.SKU{
			SKU:       skuCode,
			Name:      input.Name,
			Barcode:   strings.TrimSpace(input.Barcode),
			Category:  input.Type,
			Price:     input.RetailPrice,
			CostPrice: input.Cost,
			IsBundle:  input.IsBundle,
			Image:     input.Image,
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := u.repo.CreateSKU(txCtx, &skuEntity); err != nil {
			return err
		}

		// Determine initial stock
		initQty := input.InitialQuantity
		if initQty <= 0 && input.Stock > 0 {
			initQty = input.Stock
		}
		if initQty < 0 {
			initQty = 0
		}

		// Create initial stock row
		stockEntity := domainStock.Stock{
			SKUID:        skuEntity.ID,
			SKUCode:      skuEntity.SKU,
			WarehouseID:  1,
			Quantity:     initQty,
			AvailableQty: initQty,
			UpdatedAt:    time.Now(),
		}
		if err := u.repo.CreateStock(txCtx, &stockEntity); err != nil {
			return err
		}

		if initQty > 0 {
			movement := domainStock.StockMovement{
				SKUID:         skuEntity.ID,
				SKUCode:       skuEntity.SKU,
				WarehouseID:   1,
				Type:          domainStock.MovementIn,
				Quantity:      initQty,
				BeforeQty:     0,
				AfterQty:      initQty,
				ReferenceType: "opening_stock",
				ReferenceID:   skuEntity.SKU,
				Note:          "Initial stock on SKU creation",
				CreatedAt:     time.Now(),
			}
			if err := u.repo.CreateMovement(txCtx, &movement); err != nil {
				return err
			}
		}

		if input.IsBundle {
			for _, comp := range input.Components {
				compSKU := comp.ComponentSKU
				if compSKU == "" && comp.SKU != "" {
					compSKU = comp.SKU
				}
				compSKU = strings.ToUpper(strings.TrimSpace(compSKU))
				if compSKU == "" {
					continue
				}

				qty := comp.Quantity
				if qty <= 0 && comp.Qty > 0 {
					qty = comp.Qty
				}
				if qty <= 0 {
					qty = 1
				}

				bundleItem := domainBundle.BundleItem{
					BundleSKU:    skuEntity.SKU,
					ComponentSKU: compSKU,
					Quantity:     qty,
					Note:         comp.Note,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				if err := u.repo.CreateBundleItem(txCtx, &bundleItem); err != nil {
					return err
				}
			}
		} else if len(input.Accessories) > 0 {
			for _, acc := range input.Accessories {
				aSKU := acc.AccessorySKU
				if aSKU == "" && acc.SKU != "" {
					aSKU = acc.SKU
				}
				aSKU = strings.ToUpper(strings.TrimSpace(aSKU))
				if aSKU == "" {
					continue
				}

				qty := acc.Quantity
				if qty <= 0 && acc.Qty > 0 {
					qty = acc.Qty
				}
				if qty <= 0 {
					qty = 1
				}

				accItem := domainSKU.SKUAccessory{
					SKU:          skuEntity.SKU,
					AccessorySKU: aSKU,
					Quantity:     qty,
					Note:         acc.Note,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				if err := u.repo.CreateAccessory(txCtx, &accItem); err != nil {
					return err
				}
				createdAccessories = append(createdAccessories, SKUAccessoryRecord{
					ID:           accItem.ID,
					SKU:          accItem.SKU,
					AccessorySKU: accItem.AccessorySKU,
					Quantity:     accItem.Quantity,
					Note:         accItem.Note,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, dbErr400("Failed to create product: "+err.Error(), "")
	}

	rec := &ProductRecord{
		ID:          skuEntity.ID,
		SKU:         skuEntity.SKU,
		Name:        skuEntity.Name,
		Type:        skuEntity.Category,
		Barcode:     skuEntity.Barcode,
		BaseUnit:    input.BaseUnit,
		RetailPrice: skuEntity.Price,
		Cost:        skuEntity.CostPrice,
		IsBundle:    skuEntity.IsBundle,
		IsActive:    true,
		Image:       skuEntity.Image,
		Accessories: createdAccessories,
	}
	return rec, nil
}

// renameTarget applies the SKU rename rules shared by UpdateByID/Update:
// uniqueness check (case-insensitive, excluding this record) then the change.
func (u *usecase) renameTarget(ctx context.Context, s *domainSKU.SKU, input UpdateInput) (skuChanged bool, appErrReturn error) {
	if input.SKU == nil {
		return false, nil
	}
	newSKU := strings.ToUpper(strings.TrimSpace(*input.SKU))
	if newSKU == "" || newSKU == strings.ToUpper(s.SKU) {
		return false, nil
	}
	// Check uniqueness
	exists, _ := u.repo.ExistsUpperExcluding(ctx, newSKU, s.ID)
	if exists {
		return false, appErr("SKU_ALREADY_EXISTS", "รหัส SKU '"+newSKU+"' ถูกใช้งานแล้ว กรุณาใช้รหัสอื่น", 400)
	}
	s.SKU = newSKU
	return true, nil
}

// applyPartialFields implements FULL-11: only fields present in the payload
// are applied; omitted fields keep their stored values.
func applyPartialFields(s *domainSKU.SKU, input UpdateInput) {
	if input.Name != nil && *input.Name != "" {
		s.Name = *input.Name
	}
	if input.Barcode != nil {
		s.Barcode = strings.TrimSpace(*input.Barcode)
	}
	if input.Type != nil {
		s.Category = *input.Type
	}
	if input.RetailPrice != nil && *input.RetailPrice > 0 {
		s.Price = *input.RetailPrice
	}
	if input.Cost != nil && *input.Cost >= 0 {
		s.CostPrice = *input.Cost
	}
	// Spec: SKU ใหม่และการแก้ไข SKU ถูกบังคับเป็นสินค้าปกติ
	s.IsBundle = false
	if input.Image != nil {
		s.Image = *input.Image
	}
	if input.Status != nil && *input.Status != "" {
		s.Status = strings.ToLower(strings.TrimSpace(*input.Status))
	}
	s.UpdatedAt = time.Now()
}

// UpdateByID updates a product by numeric record ID.
func (u *usecase) UpdateByID(ctx context.Context, idStr string, input UpdateInput) (*domainSKU.SKU, error) {
	uid, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return nil, productNotFound404()
	}
	s, err := u.repo.FindByID(ctx, uint(uid))
	if err != nil {
		return nil, productNotFound404()
	}

	oldSKU := s.SKU
	skuChanged, appErrReturn := u.renameTarget(ctx, s, input)
	if appErrReturn != nil {
		return nil, appErrReturn
	}

	applyPartialFields(s, input)

	err = u.txMgr.Transaction(ctx, func(txCtx context.Context) error {
		if err := u.repo.SaveSKU(txCtx, s); err != nil {
			return err
		}
		if skuChanged {
			return u.repo.CascadeSKURename(txCtx, oldSKU, s.SKU)
		}
		return nil
	})
	if err != nil {
		return nil, dbErr500("Failed to update product: " + err.Error())
	}

	return s, nil
}

// Update updates a product strictly by SKU string (FULL-12 v2) and replaces
// bundle components / accessories when included in the payload.
func (u *usecase) Update(ctx context.Context, code string, input UpdateInput) (*ProductRecord, error) {
	s, err := u.repo.FindByUpperSKU(ctx, code)
	if err != nil {
		return nil, productNotFound404()
	}

	oldSKU := s.SKU
	skuChanged, appErrReturn := u.renameTarget(ctx, s, input)
	if appErrReturn != nil {
		return nil, appErrReturn
	}

	applyPartialFields(s, input)

	// Validate accessories if provided for non-bundle product
	if !s.IsBundle && input.Accessories != nil {
		if appErrA := u.validateAccessories(ctx, strings.ToUpper(s.SKU), input.Accessories); appErrA != nil {
			return nil, appErrA
		}
	}

	// Use the OLD sku for cleanup when SKU was renamed,
	// because cascade hasn't run yet at that point.
	cleanupSKU := oldSKU

	var updatedAccessories []SKUAccessoryRecord
	err = u.txMgr.Transaction(ctx, func(txCtx context.Context) error {
		if err := u.repo.SaveSKU(txCtx, s); err != nil {
			return err
		}

		// Cascade SKU rename to all referencing tables
		if skuChanged {
			if err := u.repo.CascadeSKURename(txCtx, oldSKU, s.SKU); err != nil {
				return err
			}
			cleanupSKU = s.SKU
		}

		if s.IsBundle {
			// If product is bundle, remove any accessories it might have previously had
			_ = u.repo.DeleteAccessoriesBySKU(txCtx, cleanupSKU)

			if input.Components != nil {
				// Replace existing bundle components with the updated list
				if err := u.repo.DeleteBundleItemsByBundleSKU(txCtx, cleanupSKU); err != nil {
					return err
				}
				for _, comp := range input.Components {
					compSKU := comp.ComponentSKU
					if compSKU == "" && comp.SKU != "" {
						compSKU = comp.SKU
					}
					compSKU = strings.ToUpper(strings.TrimSpace(compSKU))
					if compSKU == "" {
						continue
					}

					qty := comp.Quantity
					if qty <= 0 && comp.Qty > 0 {
						qty = comp.Qty
					}
					if qty <= 0 {
						qty = 1
					}

					bundleItem := domainBundle.BundleItem{
						BundleSKU:    s.SKU,
						ComponentSKU: compSKU,
						Quantity:     qty,
						Note:         comp.Note,
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					}
					if err := u.repo.CreateBundleItem(txCtx, &bundleItem); err != nil {
						return err
					}
				}
			}
		} else {
			// If product is non-bundle, remove any bundle_items it might have had
			_ = u.repo.DeleteBundleItemsByBundleSKU(txCtx, cleanupSKU)

			if input.Accessories != nil {
				// Replace existing accessories
				if err := u.repo.DeleteAccessoriesBySKU(txCtx, cleanupSKU); err != nil {
					return err
				}
				for _, acc := range input.Accessories {
					aSKU := acc.AccessorySKU
					if aSKU == "" && acc.SKU != "" {
						aSKU = acc.SKU
					}
					aSKU = strings.ToUpper(strings.TrimSpace(aSKU))
					if aSKU == "" {
						continue
					}

					qty := acc.Quantity
					if qty <= 0 && acc.Qty > 0 {
						qty = acc.Qty
					}
					if qty <= 0 {
						qty = 1
					}

					accItem := domainSKU.SKUAccessory{
						SKU:          s.SKU,
						AccessorySKU: aSKU,
						Quantity:     qty,
						Note:         acc.Note,
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					}
					if err := u.repo.CreateAccessory(txCtx, &accItem); err != nil {
						return err
					}
					updatedAccessories = append(updatedAccessories, SKUAccessoryRecord{
						ID:           accItem.ID,
						SKU:          accItem.SKU,
						AccessorySKU: accItem.AccessorySKU,
						Quantity:     accItem.Quantity,
						Note:         accItem.Note,
					})
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, dbErr500("Failed to update product: " + err.Error())
	}

	// If accessories weren't passed in update, fetch existing
	if !s.IsBundle && input.Accessories == nil {
		accItems, _ := u.repo.AccessoriesForSKU(ctx, s.SKU)
		for _, ai := range accItems {
			updatedAccessories = append(updatedAccessories, SKUAccessoryRecord{
				ID:           ai.ID,
				SKU:          ai.SKU,
				AccessorySKU: ai.AccessorySKU,
				Quantity:     ai.Quantity,
				Note:         ai.Note,
			})
		}
	}

	rec := &ProductRecord{
		ID:          s.ID,
		SKU:         s.SKU,
		Name:        s.Name,
		Type:        s.Category,
		RetailPrice: s.Price,
		Cost:        s.CostPrice,
		IsBundle:    s.IsBundle,
		IsActive:    s.Status == "active",
		Image:       s.Image,
		Accessories: updatedAccessories,
	}
	return rec, nil
}

// normalizeStatus maps free-form status values the way the old endpoints did:
// "active"/"inactive" stay, "archived" becomes inactive, anything else falls
// back to active.
func normalizeStatus(status string) string {
	newStatus := strings.ToLower(strings.TrimSpace(status))
	if newStatus != "active" && newStatus != "inactive" {
		if newStatus == "archived" {
			newStatus = "inactive"
		} else {
			newStatus = "active"
		}
	}
	return newStatus
}

// FindByID resolves a product by numeric record ID (workspace detail route).
func (u *usecase) FindByID(ctx context.Context, idStr string) (*domainSKU.SKU, error) {
	uid, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return nil, productNotFound404()
	}
	s, err := u.repo.FindByID(ctx, uint(uid))
	if err != nil {
		return nil, productNotFound404()
	}
	return s, nil
}

// SetStatusByID toggles a product's status by numeric record ID.
func (u *usecase) SetStatusByID(ctx context.Context, idStr, status string) (*domainSKU.SKU, error) {
	uid, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return nil, productNotFound404()
	}
	s, err := u.repo.FindByID(ctx, uint(uid))
	if err != nil {
		return nil, productNotFound404()
	}
	s.Status = normalizeStatus(status)
	s.UpdatedAt = time.Now()
	if err := u.repo.SaveSKU(ctx, s); err != nil {
		return nil, dbErr500("Failed to update product status")
	}
	return s, nil
}

// SetStatus toggles a product's status strictly by SKU string.
func (u *usecase) SetStatus(ctx context.Context, code, status string) (*domainSKU.SKU, error) {
	s, err := u.repo.FindByUpperSKU(ctx, code)
	if err != nil {
		return nil, productNotFound404()
	}
	s.Status = normalizeStatus(status)
	s.UpdatedAt = time.Now()
	if err := u.repo.SaveSKU(ctx, s); err != nil {
		return nil, dbErr500("Failed to update product status")
	}
	return s, nil
}

// SetReorderByID updates a product's reorder point by numeric record ID.
func (u *usecase) SetReorderByID(ctx context.Context, idStr string, reorder int) (*domainSKU.SKU, error) {
	uid, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return nil, productNotFound404()
	}
	s, err := u.repo.FindByID(ctx, uint(uid))
	if err != nil {
		return nil, productNotFound404()
	}
	s.ReorderPoint = reorder
	s.UpdatedAt = time.Now()
	if err := u.repo.SaveSKU(ctx, s); err != nil {
		return nil, dbErr500("Failed to update product reorder point")
	}
	return s, nil
}

// SetReorder updates a product's reorder point strictly by SKU string.
func (u *usecase) SetReorder(ctx context.Context, code string, reorder int) (*domainSKU.SKU, error) {
	s, err := u.repo.FindByUpperSKU(ctx, code)
	if err != nil {
		return nil, productNotFound404()
	}
	s.ReorderPoint = reorder
	s.UpdatedAt = time.Now()
	if err := u.repo.SaveSKU(ctx, s); err != nil {
		return nil, dbErr500("Failed to update product reorder point")
	}
	return s, nil
}

// DeleteByID deletes a product by numeric record ID (reference-safe).
func (u *usecase) DeleteByID(ctx context.Context, idStr string) error {
	uid, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return productNotFound404()
	}
	s, err := u.repo.FindByID(ctx, uint(uid))
	if err != nil {
		return productNotFound404()
	}
	return u.deleteProduct(ctx, s)
}

// Delete deletes a product strictly by SKU string (case-insensitive).
func (u *usecase) Delete(ctx context.Context, code string) error {
	s, err := u.repo.FindByUpperSKU(ctx, code)
	if err != nil {
		return productNotFound404()
	}
	return u.deleteProduct(ctx, s)
}

// deleteProduct deletes a resolved SKU with the FULL-13 reference check
// running in ONE transaction (row-locked) with the historical messages.
func (u *usecase) deleteProduct(ctx context.Context, s *domainSKU.SKU) error {
	var inUse bool
	var inUseMsg string
	err := u.txMgr.Transaction(ctx, func(txCtx context.Context) error {
		// Re-lock the SKU row inside the tx so the product cannot be deleted
		// twice or mutated while we inspect references.
		if _, err := u.repo.LockSKU(txCtx, s.ID); err != nil {
			return err
		}

		refs, err := u.repo.CountReferences(txCtx, s)
		if err != nil {
			return err
		}

		var inUseReasons []string
		if refs.PositiveStock > 0 {
			inUseReasons = append(inUseReasons, fmt.Sprintf("มีสต็อกคงเหลือ (%d รายการ)", refs.PositiveStock))
		}
		if refs.Movements > 0 {
			inUseReasons = append(inUseReasons, fmt.Sprintf("มีประวัติการเคลื่อนไหวสต็อก (%d รายการ)", refs.Movements))
		}
		if refs.OrderItems > 0 {
			inUseReasons = append(inUseReasons, fmt.Sprintf("อยู่ในรายการคำสั่งซื้อ (%d รายการ)", refs.OrderItems))
		}
		if refs.POItems > 0 {
			inUseReasons = append(inUseReasons, fmt.Sprintf("อยู่ในใบสั่งซื้อ PO (%d รายการ)", refs.POItems))
		}
		if refs.QuotationLn > 0 {
			inUseReasons = append(inUseReasons, fmt.Sprintf("อยู่ในใบเสนอราคา (%d รายการ)", refs.QuotationLn))
		}
		if refs.UsedAsComp > 0 {
			inUseReasons = append(inUseReasons, fmt.Sprintf("ถูกใช้เป็นส่วนประกอบในชุด Bundle อื่น (%d รายการ)", refs.UsedAsComp))
		}
		if refs.UsedAsAcc > 0 {
			inUseReasons = append(inUseReasons, fmt.Sprintf("ถูกใช้เป็นบรรจุภัณฑ์/Accessory ในสินค้าอื่น (%d รายการ)", refs.UsedAsAcc))
		}

		if len(inUseReasons) > 0 {
			inUse = true
			inUseMsg = strings.Join(inUseReasons, ", ")
			return nil
		}

		// Clean up initial zero-stock records, bundle definitions, and accessories owned by this SKU
		if err := u.repo.DeleteStocksBySKUID(txCtx, s.ID); err != nil {
			return err
		}
		if err := u.repo.DeleteBundleItemsByBundleSKU(txCtx, s.SKU); err != nil {
			return err
		}
		if err := u.repo.DeleteAccessoriesBySKU(txCtx, s.SKU); err != nil {
			return err
		}

		return u.repo.DeleteSKU(txCtx, s.ID)
	})

	if err != nil {
		return dbErr500("Failed to delete product: " + err.Error())
	}
	if inUse {
		return appErr("PRODUCT_IN_USE", "ไม่สามารถลบสินค้านี้ได้: "+inUseMsg+" กรุณาปิดการใช้งาน (Inactive) แทน", 400)
	}

	return nil
}
