package purchasing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainSeq "chawy-erp-api/internal/domain/sequence"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	"chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"

	"gorm.io/gorm"
)

// GoodsReceiveItemInput is one requested receive line. QtyReceived wins over
// Quantity when both are present, matching the legacy workspace payload.
type GoodsReceiveItemInput struct {
	SKU         string
	QtyReceived int
	Quantity    int
	UnitCost    *float64
	RetailPrice *float64
	ExpiryDate  string
	SupplierLot string
	QCStatus    string
}

// CreateGoodsReceiveInput mirrors the goods-receive screen payload.
type CreateGoodsReceiveInput struct {
	PORef       string
	ReceiveDate string
	Note        string
	Items       []GoodsReceiveItemInput
}

// CreateGoodsIssueInput mirrors the goods-issue screen payload.
type CreateGoodsIssueInput struct {
	SKU      string
	Qty      int
	Reason   string
	Channel  string
	OrderRef string
}

type receiveItem struct {
	SKU         string
	Quantity    int
	UnitCost    float64
	RetailPrice float64
	SupplierLot string
	ExpiryDate  string
	QCStatus    string
}

// CreateGoodsReceive records a goods receipt (with or without a PO), creates
// stock lots and movements, and advances PO fulfillment. The whole workflow
// runs in one transaction so a partial failure leaves no trace.
func (u *purchasingUsecase) CreateGoodsReceive(ctx context.Context, in CreateGoodsReceiveInput) (*domainPurchasing.GoodsReceive, error) {
	receiveDate := in.ReceiveDate
	if receiveDate == "" {
		receiveDate = time.Now().Format("2006-01-02")
	}

	hasPO := strings.TrimSpace(in.PORef) != ""
	var po *domainPurchasing.PurchaseOrder
	if hasPO {
		existing, err := u.purchasingRepo.FindPOByRef(ctx, in.PORef)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return nil, appErrors.NewAppError("PO_NOT_FOUND", "Purchase order not found", 404)
		}
		if existing.Status == domainPurchasing.StatusReceived {
			return nil, appErrors.NewAppError("PO_ALREADY_RECEIVED",
				fmt.Sprintf("Purchase order %s has already been received in full", existing.PONo), 400)
		}
		if existing.Status == domainPurchasing.StatusCancelled {
			return nil, appErrors.NewAppError("PO_CANCELLED",
				fmt.Sprintf("Purchase order %s is cancelled", existing.PONo), 400)
		}
		po = existing
	}

	var itemsToReceive []receiveItem

	if len(in.Items) > 0 {
		for _, it := range in.Items {
			qty := it.QtyReceived
			if qty <= 0 && it.Quantity > 0 {
				qty = it.Quantity
			}
			if it.SKU == "" || qty <= 0 {
				continue
			}

			qc := it.QCStatus
			if qc == "" {
				qc = "Accepted"
			}
			expiry := strings.TrimSpace(it.ExpiryDate)
			if expiry == "" {
				if parsedDate, err := time.Parse("2006-01-02", receiveDate); err == nil {
					expiry = parsedDate.AddDate(1, 6, 0).Format("2006-01-02")
				} else {
					expiry = time.Now().AddDate(1, 6, 0).Format("2006-01-02")
				}
			}

			unitCost := 0.0
			if it.UnitCost != nil {
				unitCost = *it.UnitCost
			} else if hasPO {
				// Fallback to PO unit cost if not explicitly passed.
				for _, poi := range po.Items {
					if strings.EqualFold(poi.SKU, it.SKU) && poi.UnitCost > 0 {
						unitCost = poi.UnitCost
						break
					}
				}
			}

			if unitCost <= 0 {
				return nil, appErrors.NewAppError("UNIT_COST_REQUIRED",
					fmt.Sprintf("กรุณาระบุราคาต้นทุนต่อหน่วยที่มากกว่า 0 สำหรับ SKU %s", it.SKU), 400)
			}

			retailPrice := 0.0
			if it.RetailPrice != nil {
				retailPrice = *it.RetailPrice
			}

			itemsToReceive = append(itemsToReceive, receiveItem{
				SKU:         strings.ToUpper(strings.TrimSpace(it.SKU)),
				Quantity:    qty,
				UnitCost:    unitCost,
				RetailPrice: retailPrice,
				SupplierLot: it.SupplierLot,
				ExpiryDate:  expiry,
				QCStatus:    qc,
			})
		}
	} else if hasPO {
		defaultExpiry := time.Now().AddDate(1, 6, 0).Format("2006-01-02")
		if parsedDate, err := time.Parse("2006-01-02", receiveDate); err == nil {
			defaultExpiry = parsedDate.AddDate(1, 6, 0).Format("2006-01-02")
		}
		for _, it := range po.Items {
			remaining := it.Quantity - it.ReceivedQty
			if remaining > 0 {
				itemsToReceive = append(itemsToReceive, receiveItem{
					SKU:         strings.ToUpper(strings.TrimSpace(it.SKU)),
					Quantity:    remaining,
					UnitCost:    it.UnitCost,
					RetailPrice: 0,
					ExpiryDate:  defaultExpiry,
					QCStatus:    "Accepted",
				})
			}
		}
	}

	if len(itemsToReceive) == 0 {
		return nil, appErrors.NewAppError("NOTHING_TO_RECEIVE", "No remaining items to receive", 400)
	}

	var createdGR *domainPurchasing.GoodsReceive

	run := func(txCtx context.Context) error {
		var poIDPtr *uint
		supplierName := ""

		if hasPO {
			// Lock the PO row to prevent concurrent double receive.
			lockedPO, err := u.purchasingRepo.FindPOByIDForUpdate(txCtx, po.ID)
			if err != nil {
				return err
			}
			if lockedPO == nil {
				return errors.New("purchase order not found")
			}
			if lockedPO.Status == domainPurchasing.StatusReceived {
				return errors.New("Purchase order has already been received")
			}

			poIDPtr = &lockedPO.ID
			supplierName = lockedPO.SupplierName

			// FULL-14: map PO lines to a slice so duplicate SKUs on separate
			// lines are honored instead of collapsing to the last line.
			poLineIndex := make(map[string][]*domainPurchasing.POItem)
			for i := range lockedPO.Items {
				skuKey := strings.ToUpper(strings.TrimSpace(lockedPO.Items[i].SKU))
				poLineIndex[skuKey] = append(poLineIndex[skuKey], &lockedPO.Items[i])
			}

			// FULL-10: QC-rejected units are validated against the PO but do
			// not count as received for fulfillment purposes.
			receivedAgg := make(map[string]int)
			for _, recIt := range itemsToReceive {
				lines, exists := poLineIndex[recIt.SKU]
				if !exists || len(lines) == 0 {
					return fmt.Errorf("SKU %s does not belong to purchase order %s", recIt.SKU, lockedPO.PONo)
				}
				remaining := 0
				for _, poIt := range lines {
					remaining += poIt.Quantity - poIt.ReceivedQty
				}
				receivedAgg[recIt.SKU] += recIt.Quantity
				if receivedAgg[recIt.SKU] > remaining {
					return fmt.Errorf("quantity to receive for SKU %s (%d) exceeds remaining PO quantity (%d)", recIt.SKU, receivedAgg[recIt.SKU], remaining)
				}
			}

			receivedAccepted := make(map[string]int)
			for _, recIt := range itemsToReceive {
				qcUpper := strings.ToUpper(strings.TrimSpace(recIt.QCStatus))
				if qcUpper == "" || qcUpper == "ACCEPTED" || qcUpper == "PASS" || qcUpper == "PASSED" {
					receivedAccepted[recIt.SKU] += recIt.Quantity
				}
			}

			// FULL-14: distribute accepted quantities across duplicate PO lines
			// in order, filling each line's remaining amount first.
			for skuKey, qtyAdded := range receivedAccepted {
				left := qtyAdded
				for _, poIt := range poLineIndex[skuKey] {
					if left <= 0 {
						break
					}
					lineRemaining := poIt.Quantity - poIt.ReceivedQty
					if lineRemaining <= 0 {
						continue
					}
					take := lineRemaining
					if left < take {
						take = left
					}
					poIt.ReceivedQty += take
					left -= take
					if err := u.purchasingRepo.UpdatePOItemReceivedQty(txCtx, poIt.ID, poIt.ReceivedQty); err != nil {
						return err
					}
				}
			}

			allDone := true
			for i := range lockedPO.Items {
				if lockedPO.Items[i].ReceivedQty < lockedPO.Items[i].Quantity {
					allDone = false
					break
				}
			}

			if allDone {
				lockedPO.Status = domainPurchasing.StatusReceived
			}
			lockedPO.UpdatedAt = time.Now()
			if in.Note != "" {
				lockedPO.Note = in.Note
			}
			if err := u.purchasingRepo.UpdatePO(txCtx, lockedPO); err != nil {
				return err
			}
			po = lockedPO
		}

		grCode := fmt.Sprintf("GR-%s-%04d", time.Now().Format("2006/01/02"), time.Now().UnixNano()%10000)
		if u.seqUsecase != nil {
			var docDate *time.Time
			if parsed, err := time.Parse("2006-01-02", receiveDate); err == nil {
				docDate = &parsed
			}
			if genCode, err := u.seqUsecase.Generate(txCtx, string(domainSeq.TypeGoodsReceive), docDate); err == nil && genCode != "" {
				grCode = genCode
			}
		}

		gr := &domainPurchasing.GoodsReceive{
			Code:         grCode,
			POID:         poIDPtr,
			PORef:        in.PORef,
			SupplierName: supplierName,
			ReceiveDate:  receiveDate,
			Note:         in.Note,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		// Resolve every SKU and stage the GR lines first: the repository writes
		// header + lines together and we need the line IDs for lot numbering.
		grItems := make([]domainPurchasing.GoodsReceiveItem, 0, len(itemsToReceive))
		skus := make([]*domainSKU.SKU, len(itemsToReceive))
		for i, it := range itemsToReceive {
			s, err := u.findSKUByCode(txCtx, it.SKU)
			if err != nil {
				return err
			}
			if s == nil {
				return fmt.Errorf("SKU %s not found in system", it.SKU)
			}
			skus[i] = s
			grItems = append(grItems, domainPurchasing.GoodsReceiveItem{
				SKU:         s.SKU,
				Name:        s.Name,
				Quantity:    it.Quantity,
				UnitCost:    it.UnitCost,
				RetailPrice: it.RetailPrice,
				SupplierLot: it.SupplierLot,
				ExpiryDate:  it.ExpiryDate,
				QCStatus:    it.QCStatus,
				CreatedAt:   time.Now(),
			})
		}

		if err := u.purchasingRepo.CreateGoodsReceiveDoc(txCtx, gr, grItems); err != nil {
			return err
		}
		createdGR = gr

		for i, it := range itemsToReceive {
			s := skus[i]

			// FULL-10: only QC-accepted units become sellable stock. Rejected
			// units stay on the GR for traceability.
			qcUpper := strings.ToUpper(strings.TrimSpace(it.QCStatus))
			qcAccepted := qcUpper == "" || qcUpper == "ACCEPTED" || qcUpper == "PASS" || qcUpper == "PASSED"

			stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, s.ID, 1)
			if err != nil {
				return err
			}
			beforeQty := 0
			if stk != nil {
				beforeQty = stk.Quantity
			}
			afterQty := beforeQty
			if qcAccepted {
				updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, s.ID, 1, it.Quantity)
				if err != nil {
					return err
				}
				afterQty = updatedStk.Quantity
			}

			var stockLotID *uint
			if qcAccepted {
				lotNumber := it.SupplierLot
				if lotNumber == "" {
					lotNumber = fmt.Sprintf("LOT-%s-%d", time.Now().Format("20060102"), grItems[i].ID)
				}

				stockLot := &domainStock.StockLot{
					SKUID:        s.ID,
					SKUCode:      s.SKU,
					WarehouseID:  1,
					LotNumber:    lotNumber,
					SupplierLot:  it.SupplierLot,
					ExpiryDate:   it.ExpiryDate,
					Quantity:     it.Quantity,
					AvailableQty: it.Quantity,
					UnitCost:     it.UnitCost,
					RetailPrice:  it.RetailPrice,
					ReceivedAt:   time.Now(),
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				if err := u.stockRepo.CreateLot(txCtx, stockLot); err != nil {
					return err
				}
				stockLotID = &stockLot.ID

				// Update SKU master with latest unit cost and selling price.
				if it.UnitCost > 0 {
					s.CostPrice = it.UnitCost
				}
				if it.RetailPrice > 0 {
					s.Price = it.RetailPrice
				}
				s.UpdatedAt = time.Now()
				if err := u.skuRepo.Update(txCtx, s); err != nil {
					return err
				}
			}

			note := in.Note
			if it.SupplierLot != "" || it.ExpiryDate != "" {
				note = fmt.Sprintf("%s [Lot: %s, Exp: %s]", note, it.SupplierLot, it.ExpiryDate)
			}
			if it.UnitCost > 0 || it.RetailPrice > 0 {
				note = fmt.Sprintf("%s [Cost: %.2f, Price: %.2f]", note, it.UnitCost, it.RetailPrice)
			}
			if !qcAccepted {
				if note != "" {
					note += " "
				}
				note += fmt.Sprintf("[QC %s — not added to sellable stock]", it.QCStatus)
			}

			// Rejected units still generate a movement trail, but as a
			// QC_REJECT type with zero stock effect.
			movementType := domainStock.MovementIn
			movementQty := it.Quantity
			if !qcAccepted {
				movementType = domainStock.MovementType("QC_REJECT")
				movementQty = 0
			}

			movement := &domainStock.StockMovement{
				SKUID:         s.ID,
				SKUCode:       s.SKU,
				WarehouseID:   1,
				StockLotID:    stockLotID,
				Type:          movementType,
				Quantity:      movementQty,
				BeforeQty:     beforeQty,
				AfterQty:      afterQty,
				ReferenceType: "GOODS_RECEIVE",
				ReferenceID:   gr.Code,
				Note:          strings.TrimSpace(note),
				CreatedAt:     time.Now(),
			}
			if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
				return err
			}
		}

		return nil
	}

	if err := u.withTx(ctx, run); err != nil {
		return nil, err
	}

	return createdGR, nil
}

// CreateGoodsIssue deducts stock for a manual issue and records the movement.
func (u *purchasingUsecase) CreateGoodsIssue(ctx context.Context, in CreateGoodsIssueInput) (*domainStock.StockMovement, error) {
	skuCode := strings.ToUpper(strings.TrimSpace(in.SKU))
	s, err := u.findSKUByCode(ctx, skuCode)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, appErrors.NewAppError("SKU_NOT_FOUND", "SKU not found", 404)
	}

	refType := in.Channel
	if refType == "" {
		refType = "MANUAL_ISSUE"
	}

	var movement *domainStock.StockMovement

	run := func(txCtx context.Context) error {
		stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, s.ID, 1)
		if err != nil {
			return err
		}
		if stk == nil {
			return errors.New("Stock not found for SKU")
		}
		if stk.Quantity < in.Qty {
			return fmt.Errorf("insufficient total stock: current %d, required %d", stk.Quantity, in.Qty)
		}
		if stk.AvailableQty < in.Qty {
			return fmt.Errorf("insufficient available stock (allocated for reservations): available %d, required %d", stk.AvailableQty, in.Qty)
		}

		beforeQty := stk.Quantity
		updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, s.ID, 1, -in.Qty)
		if err != nil {
			return err
		}

		movement = &domainStock.StockMovement{
			SKUID:         s.ID,
			SKUCode:       s.SKU,
			WarehouseID:   1,
			Type:          domainStock.MovementOut,
			Quantity:      in.Qty,
			BeforeQty:     beforeQty,
			AfterQty:      updatedStk.Quantity,
			ReferenceType: refType,
			ReferenceID:   in.OrderRef,
			Note:          in.Reason,
			CreatedAt:     time.Now(),
		}
		return u.stockRepo.CreateMovement(txCtx, movement)
	}

	if err := u.withTx(ctx, run); err != nil {
		return nil, err
	}

	return movement, nil
}

// findSKUByCode resolves a SKU code case-insensitively (and ignoring
// surrounding spaces), matching the UPPER(TRIM(sku)) lookup the workspace
// screens used before the logic moved into this usecase.
func (u *purchasingUsecase) findSKUByCode(ctx context.Context, code string) (*domainSKU.SKU, error) {
	s, err := u.skuRepo.FindBySKU(ctx, code)
	if err != nil || s != nil {
		return s, err
	}
	// Fall back to a search so rows stored with different casing still match.
	items, _, err := u.skuRepo.FindAll(ctx, domainSKU.Query{Search: code, Page: 1, Limit: 20})
	if err != nil {
		return nil, err
	}
	want := strings.ToUpper(strings.TrimSpace(code))
	for i := range items {
		if strings.ToUpper(strings.TrimSpace(items[i].SKU)) == want {
			return &items[i], nil
		}
	}
	return nil, nil
}

// withTx runs fn inside the configured transaction manager. It falls back to a
// direct GORM transaction when no manager was injected (tests, legacy wiring).
func (u *purchasingUsecase) withTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if u.txMgr != nil {
		return u.txMgr.Transaction(ctx, fn)
	}
	if u.db == nil {
		return fn(ctx)
	}
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(database.WithTxContext(ctx, tx))
	})
}
