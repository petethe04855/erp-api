package handlers

import (
	"fmt"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// POST /api/purchase-requests
func CreatePurchaseRequest(c *fiber.Ctx) error {
	var pr models.PurchaseRequest
	if err := c.BodyParser(&pr); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if pr.Requester == "" || pr.NeededDate == "" || len(pr.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PR requires requester, neededDate, and at least one item"})
	}

	for _, item := range pr.Items {
		if item.Qty <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "All PR items must have qty > 0"})
		}
		if err := validatePurchasableItem(database.DB, item.SKU); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "PR-2026-", &models.PurchaseRequest{}, "code")
		if err != nil {
			return err
		}
		pr.Code = code
		pr.Date = time.Now().Format("2006-01-02")
		pr.Status = "Pending Approval"

		if err := tx.Create(&pr).Error; err != nil {
			return err
		}

		for i := range pr.Items {
			pr.Items[i].PurchaseRequestID = pr.ID
			tx.Model(&pr.Items[i]).Update("purchase_request_id", pr.ID)
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").First(&pr, pr.ID)
	return c.JSON(pr)
}

// PUT /api/purchase-requests/:id/status
func UpdatePRStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var pr models.PurchaseRequest
	if err := ByIDOrCode(database.DB, id).First(&pr).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "PR not found"})
	}

	pr.Status = req.Status
	if err := database.DB.Save(&pr).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").First(&pr, pr.ID)
	return c.JSON(pr)
}

type ConvertPRtoPORequest struct {
	Supplier  string             `json:"supplier"`
	EtaDate   string             `json:"etaDate"`
	ItemCosts map[string]float64 `json:"itemCosts"`
}

// POST /api/purchase-requests/:id/convert
func ConvertPRtoPO(c *fiber.Ctx) error {
	id := c.Params("id")
	var pr models.PurchaseRequest
	if err := ByIDOrCode(database.DB, id).Preload("Items").First(&pr).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "PR not found"})
	}

	if pr.Status != "Approved" || pr.PoRef != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PR must be Approved and not converted already"})
	}

	var req ConvertPRtoPORequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	var po models.PurchaseOrder
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "PO-2026-", &models.PurchaseOrder{}, "code")
		if err != nil {
			return err
		}

		po = models.PurchaseOrder{
			Code:              code,
			Supplier:          req.Supplier,
			EtaDate:           req.EtaDate,
			Date:              time.Now().Format("2006-01-02"),
			Status:            "Draft",
			PurchaseRequestID: &pr.ID,
			PrRef:             pr.Code,
		}

		if err := tx.Create(&po).Error; err != nil {
			return err
		}

		var poItems []models.PurchaseOrderItem
		var totalCost float64
		for _, item := range pr.Items {
			cost := req.ItemCosts[item.SKU]
			poItem := models.PurchaseOrderItem{
				PurchaseOrderID: po.ID,
				ProductID:       item.ProductID,
				SKU:             item.SKU,
				Name:            item.Name,
				Qty:             item.Qty,
				UnitCost:        cost,
				ReceivedQty:     0,
			}
			if err := tx.Create(&poItem).Error; err != nil {
				return err
			}
			poItems = append(poItems, poItem)
			totalCost += float64(item.Qty) * cost
		}

		po.Items = poItems
		po.TotalCost = totalCost
		if err := tx.Save(&po).Error; err != nil {
			return err
		}

		// Update PR poRef
		pr.PoRef = po.Code
		if err := tx.Save(&pr).Error; err != nil {
			return err
		}

		// Create Audit Event
		audit := models.AuditEvent{
			OwnerID:   po.ID,
			OwnerType: "purchase_orders",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("แปลงจาก %s", pr.Code),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").Preload("AuditTrail").First(&po, po.ID)
	return c.JSON(po)
}

// POST /api/purchase-orders
func CreatePurchaseOrder(c *fiber.Ctx) error {
	var po models.PurchaseOrder
	if err := c.BodyParser(&po); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if po.Supplier == "" || po.EtaDate == "" || len(po.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PO requires supplier, etaDate, and at least one item"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "PO-2026-", &models.PurchaseOrder{}, "code")
		if err != nil {
			return err
		}
		po.Code = code
		po.Date = time.Now().Format("2006-01-02")
		po.Status = "Draft"

		if err := tx.Create(&po).Error; err != nil {
			return err
		}

		var totalCost float64
		for i := range po.Items {
			po.Items[i].PurchaseOrderID = po.ID
			po.Items[i].ReceivedQty = 0
			if err := validatePurchasableItem(tx, po.Items[i].SKU); err != nil {
				return err
			}
			tx.Model(&po.Items[i]).Update("purchase_order_id", po.ID)
			totalCost += float64(po.Items[i].Qty) * po.Items[i].UnitCost
		}
		po.TotalCost = totalCost
		if err := tx.Save(&po).Error; err != nil {
			return err
		}

		audit := models.AuditEvent{
			OwnerID:   po.ID,
			OwnerType: "purchase_orders",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      "สร้าง Purchase Order",
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").Preload("AuditTrail").First(&po, po.ID)
	return c.JSON(po)
}

// PUT /api/purchase-orders/:id/status
func UpdatePOStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var po models.PurchaseOrder
	if err := ByIDOrCode(database.DB, id).Preload("Items").First(&po).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "PO not found"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		oldStatus := po.Status
		po.Status = req.Status
		if err := tx.Save(&po).Error; err != nil {
			return err
		}

		audit := models.AuditEvent{
			OwnerID:   po.ID,
			OwnerType: "purchase_orders",
			Action:    req.Status,
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("เปลี่ยนสถานะจาก %s เป็น %s", oldStatus, req.Status),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").Preload("AuditTrail").First(&po, po.ID)
	return c.JSON(po)
}

// POST /api/goods-receives
func CreateGoodsReceive(c *fiber.Ctx) error {
	var gr models.GoodsReceive
	if err := c.BodyParser(&gr); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if gr.ReceiveDate == "" || len(gr.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Stock receipt requires receiveDate and at least one item"})
	}

	hasPO := gr.PoRef != "" || gr.PurchaseOrderID != nil
	var po models.PurchaseOrder
	if hasPO {
		if err := ByIDOrCode(database.DB, gr.PoRef).Preload("Items").First(&po).Error; err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Matching PO not found"})
		}
		if po.Status != "Sent" && po.Status != "Partial Received" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PO must be Sent or Partial Received status to receive goods"})
		}
		gr.PurchaseOrderID = &po.ID
		gr.PoRef = po.Code
	} else {
		gr.PurchaseOrderID = nil
		gr.PoRef = ""
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "GR-2026-", &models.GoodsReceive{}, "code")
		if err != nil {
			return err
		}
		gr.Code = code

		if err := tx.Create(&gr).Error; err != nil {
			return err
		}

		// Assign GoodsReceiveID to LandedCosts
		for idx := range gr.LandedCosts {
			gr.LandedCosts[idx].GoodsReceiveID = gr.ID
			tx.Model(&gr.LandedCosts[idx]).Update("goods_receive_id", gr.ID)
		}

		// รวมค่า landed ที่ allocatable
		totalLanded := 0.0
		for _, lc := range gr.LandedCosts {
			if lc.Allocatable {
				totalLanded += lc.Amount
			}
		}

		// มูลค่ารวมของที่รับ ใช้ราคา PO หรือราคาต่อหน่วยที่กรอกโดยตรง
		totalValue := 0.0
		receiptValue := 0.0
		journalLines := make([]postingLine, 0, len(gr.Items)*2)
		for i := range gr.Items {
			unitCost := gr.Items[i].LandedUnitCost
			if hasPO {
				for j := range po.Items {
					if po.Items[j].SKU == gr.Items[i].SKU {
						unitCost = po.Items[j].UnitCost
						break
					}
				}
			}
			totalValue += float64(gr.Items[i].QtyReceived) * unitCost
		}

		for i := range gr.Items {
			item := &gr.Items[i]
			item.GoodsReceiveID = gr.ID

			if item.QtyReceived <= 0 || item.Lot == "" {
				return fmt.Errorf("Invalid QtyReceived for item %s", item.SKU)
			}
			if item.LandedUnitCost < 0 {
				return fmt.Errorf("unit cost cannot be negative for item %s", item.SKU)
			}

			var product models.Product
			if err := tx.First(&product, "sku = ?", item.SKU).Error; err != nil {
				return fmt.Errorf("finished product %s not found in Item Master", item.SKU)
			}
			if !hasPO && !isSellableProduct(product) {
				return fmt.Errorf("stock receipt item %s must be a Finished Product", item.SKU)
			}
			item.ProductID = product.ID

			var duplicateLot int64
			if err := tx.Model(&models.StockLot{}).Where("sku = ? AND lot = ?", item.SKU, item.Lot).Count(&duplicateLot).Error; err != nil {
				return err
			}
			if duplicateLot > 0 {
				return fmt.Errorf("lot %s already exists for item %s", item.Lot, item.SKU)
			}

			var poItem *models.PurchaseOrderItem
			baseUnitCost := item.LandedUnitCost
			if hasPO {
				for j := range po.Items {
					if po.Items[j].SKU == item.SKU {
						poItem = &po.Items[j]
						break
					}
				}
				if poItem == nil {
					return fmt.Errorf("Item %s not found in PO %s", item.SKU, gr.PoRef)
				}
				if err := validatePurchasableItem(tx, item.SKU); err != nil {
					return err
				}
				if item.QtyReceived > (poItem.Qty - poItem.ReceivedQty) {
					return fmt.Errorf("Cannot receive more than ordered qty for item %s", item.SKU)
				}
				baseUnitCost = poItem.UnitCost
			}

			lineValue := float64(item.QtyReceived) * baseUnitCost

			// ค่าขนส่งที่ปันมาที่บรรทัดนี้
			allocatedFreight := 0.0
			if totalValue > 0 {
				allocatedFreight = totalLanded * (lineValue / totalValue)
			}

			// ต้นทุนรวมต่อหน่วย = (ราคาซื้อ + ค่าขนส่งปัน) / จำนวน
			item.LandedUnitCost = (lineValue + allocatedFreight) / float64(item.QtyReceived)
			itemValue := item.LandedUnitCost * float64(item.QtyReceived)
			receiptValue += itemValue
			if itemValue > 0 {
				journalLines = append(journalLines,
					postingLine{AccountCode: "1300", Debit: itemValue, SKU: item.SKU, Lot: item.Lot},
					postingLine{AccountCode: "2000", Credit: itemValue, SKU: item.SKU, Lot: item.Lot},
				)
			}
			if err := tx.Save(item).Error; err != nil {
				return err
			}

			if hasPO {
				poItem.ReceivedQty += item.QtyReceived
				if err := tx.Save(poItem).Error; err != nil {
					return err
				}
			}

			// Create Stock Lot
			lot := models.StockLot{
				Code:            fmt.Sprintf("LOT-%d-%s", time.Now().UnixNano(), item.Lot),
				ProductID:       product.ID,
				SKU:             item.SKU,
				Lot:             item.Lot,
				Qty:             item.QtyReceived,
				RemainingQty:    item.QtyReceived,
				LandedUnitCost:  item.LandedUnitCost,
				ExpiryDate:      item.ExpiryDate,
				ReceivedDate:    gr.ReceiveDate,
				GoodsReceiveID:  &gr.ID,
				GrRef:           gr.Code,
				PurchaseOrderID: gr.PurchaseOrderID,
				PoRef:           gr.PoRef,
			}
			if err := tx.Create(&lot).Error; err != nil {
				return err
			}

			// Update product stock balance
			if product.ID > 0 {
				oldStock := product.Stock
				if oldStock+item.QtyReceived > 0 {
					product.Cost = ((float64(oldStock) * product.Cost) + (float64(item.QtyReceived) * item.LandedUnitCost)) /
						float64(oldStock+item.QtyReceived)
				}
				product.Stock += item.QtyReceived
				if err := tx.Save(&product).Error; err != nil {
					return err
				}
			}

			// Create Stock Movement
			movement := models.StockMovement{
				Code:       fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), item.SKU),
				ProductID:  product.ID,
				SKU:        item.SKU,
				Type:       "IN",
				Qty:        item.QtyReceived,
				RefDoc:     gr.Code,
				RefDocType: "goods_receives",
				RefDocID:   &gr.ID,
				Date:       gr.ReceiveDate,
				Note:       fmt.Sprintf("รับสินค้าสำเร็จรูปเข้าคลัง lot %s exp %s", item.Lot, item.ExpiryDate),
				ChangedBy:  username.(string),
			}
			if err := tx.Create(&movement).Error; err != nil {
				return err
			}
		}

		newStatus := ""
		if hasPO {
			allCompleted := true
			for _, pi := range po.Items {
				if pi.ReceivedQty < pi.Qty {
					allCompleted = false
					break
				}
			}
			newStatus = "Partial Received"
			if allCompleted {
				newStatus = "Completed"
			}
			po.Status = newStatus
			if err := tx.Save(&po).Error; err != nil {
				return err
			}
		}

		if receiptValue > 0 {
			if _, err := postJournal(tx, postingRequest{
				Date: gr.ReceiveDate, SourceType: "goods_receipt", SourceID: gr.ID, SourceRef: gr.Code,
				Description: "รับสินค้าสำเร็จรูปเข้าคลัง", CreatedBy: username.(string), Lines: journalLines,
			}); err != nil {
				return err
			}
		}

		// Add audit trail for Goods Receive
		grAudit := models.AuditEvent{
			OwnerID:   gr.ID,
			OwnerType: "goods_receives",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      "รับสินค้าสำเร็จรูปเข้าคลัง",
		}
		if err := tx.Create(&grAudit).Error; err != nil {
			return err
		}

		if hasPO {
			poAudit := models.AuditEvent{
				OwnerID: po.ID, OwnerType: "purchase_orders", Action: newStatus,
				By: username.(string), At: getNowStr(), Note: fmt.Sprintf("รับสินค้า %s", gr.Code),
			}
			if err := tx.Create(&poAudit).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").Preload("LandedCosts").Preload("AuditTrail").First(&gr, gr.ID)
	return c.JSON(gr)
}

func recalculateProductBOMCost(tx *gorm.DB, parentSku string) error {
	var parentProduct models.Product
	if err := tx.First(&parentProduct, "sku = ?", parentSku).Error; err != nil {
		return err
	}

	var comps []models.BundleComponent
	if err := tx.Where("bundle_sku = ?", parentSku).Find(&comps).Error; err != nil {
		return err
	}

	cost, err := calculateBOMCost(tx, comps)
	if err != nil {
		return err
	}

	parentProduct.Cost = cost

	return tx.Save(&parentProduct).Error
}

func validatePurchasableItem(tx *gorm.DB, sku string) error {
	var product models.Product
	if err := tx.First(&product, "sku = ?", sku).Error; err != nil {
		return fmt.Errorf("purchase item %s not found in Item Master", sku)
	}
	if product.Type != "Raw Material" && product.Type != "Packaging" {
		return fmt.Errorf("purchase item %s must be Raw Material or Packaging", sku)
	}
	return nil
}
