package order

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainFinance "chawy-erp-api/internal/domain/finance"
	domainFormula "chawy-erp-api/internal/domain/formula"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainSeq "chawy-erp-api/internal/domain/sequence"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecaseSeq "chawy-erp-api/internal/usecase/sequence"
	usecaseStock "chawy-erp-api/internal/usecase/stock"
	"chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"

	"gorm.io/gorm"
)


type CreateItemInput struct {
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type CreateOrderInput struct {
	CustomerID   uint              `json:"customer_id"`
	CustomerName string            `json:"customer_name"`
	Channel      string            `json:"channel"`
	Note         string            `json:"note"`
	Items        []CreateItemInput `json:"items"`
	// VATIncluded marks the order as VAT-inclusive; the Usecase computes the
	// final total so no caller can overwrite amounts afterwards (FULL-17).
	VATIncluded bool `json:"vat_included"`
}

type Usecase interface {
	Create(ctx context.Context, input CreateOrderInput) (*domainOrder.Order, error)
	GetByID(ctx context.Context, id uint) (*domainOrder.Order, error)
	List(ctx context.Context, query domainOrder.Query) ([]domainOrder.Order, int64, error)
	// ListSales returns the workspace sales-order list rows together with the
	// linked invoice status (used as the "invRef" column).
	ListSales(ctx context.Context, query domainOrder.SalesQuery) ([]SalesListItem, int64, error)
	// GetByRef resolves an order by numeric id or document number.
	GetByRef(ctx context.Context, ref string) (*domainOrder.Order, error)
	// UpdateStatusByRef applies the workspace status transition rules. The
	// returned bool is true when the order already had the target status.
	UpdateStatusByRef(ctx context.Context, ref, status, note string) (*domainOrder.Order, bool, error)
	ShipOrder(ctx context.Context, id uint, warehouseID uint) (*domainOrder.Order, error)
	CancelOrder(ctx context.Context, id uint) (*domainOrder.Order, error)
}

type orderUsecase struct {
	db          *gorm.DB
	orderRepo   domainOrder.Repository
	skuRepo     domainSKU.Repository
	bundleRepo  domainBundle.Repository
	formulaRepo domainFormula.Repository
	stockRepo   domainStock.Repository
	financeRepo domainFinance.Repository
	txMgr       database.TxManager
	seqUsecase  usecaseSeq.Usecase
}

func NewOrderUsecase(
	db *gorm.DB,
	orderRepo domainOrder.Repository,
	skuRepo domainSKU.Repository,
	bundleRepo domainBundle.Repository,
	formulaRepo domainFormula.Repository,
	stockRepo domainStock.Repository,
	financeRepo domainFinance.Repository,
	seqUsecase ...usecaseSeq.Usecase,
) Usecase {
	var txMgr database.TxManager
	if db != nil {
		txMgr = database.NewTxManager(db)
	}
	var su usecaseSeq.Usecase
	if len(seqUsecase) > 0 {
		su = seqUsecase[0]
	}
	return &orderUsecase{
		db:          db,
		orderRepo:   orderRepo,
		skuRepo:     skuRepo,
		bundleRepo:  bundleRepo,
		formulaRepo: formulaRepo,
		stockRepo:   stockRepo,
		financeRepo: financeRepo,
		txMgr:       txMgr,
		seqUsecase:  su,
	}
}

func NewOrderUsecaseWithTx(
	db *gorm.DB,
	orderRepo domainOrder.Repository,
	skuRepo domainSKU.Repository,
	bundleRepo domainBundle.Repository,
	formulaRepo domainFormula.Repository,
	stockRepo domainStock.Repository,
	txMgr database.TxManager,
	seqUsecase ...usecaseSeq.Usecase,
) Usecase {
	var su usecaseSeq.Usecase
	if len(seqUsecase) > 0 {
		su = seqUsecase[0]
	}
	return &orderUsecase{
		db:          db,
		orderRepo:   orderRepo,
		skuRepo:     skuRepo,
		bundleRepo:  bundleRepo,
		formulaRepo: formulaRepo,
		stockRepo:   stockRepo,
		txMgr:       txMgr,
		seqUsecase:  su,
	}
}

func (u *orderUsecase) SetFinanceRepo(repo domainFinance.Repository) {
	u.financeRepo = repo
}


func (u *orderUsecase) withTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	if u.txMgr != nil {
		return u.txMgr.Transaction(ctx, fn)
	}
	if u.db != nil {
		return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return fn(database.WithTxContext(ctx, tx))
		})
	}
	return fn(ctx)
}

func (u *orderUsecase) Create(ctx context.Context, in CreateOrderInput) (*domainOrder.Order, error) {
	if len(in.Items) == 0 {
		return nil, appErrors.NewAppError("EMPTY_ORDER", "Order must contain at least one item", 400)
	}

	orderNo := fmt.Sprintf("SO-%s-%04d", time.Now().Format("2006/01/02"), time.Now().UnixNano()%10000)
	if u.seqUsecase != nil {
		if genNo, err := u.seqUsecase.Generate(ctx, string(domainSeq.TypeSalesOrder), nil); err == nil && genNo != "" {
			orderNo = genNo
		}
	}

	var totalAmount float64
	var items []domainOrder.OrderItem


	for _, itemInput := range in.Items {
		if itemInput.Quantity <= 0 {
			return nil, appErrors.NewAppError("INVALID_QUANTITY", fmt.Sprintf("Quantity for SKU %s must be greater than 0", itemInput.SKU), 400)
		}
		if itemInput.Price < 0 {
			return nil, appErrors.NewAppError("INVALID_PRICE", fmt.Sprintf("Price for SKU %s cannot be negative", itemInput.SKU), 400)
		}

		skuItem, err := u.skuRepo.FindBySKU(ctx, itemInput.SKU)
		if err != nil {
			return nil, err
		}

		var itemName string
		var itemPrice float64
		var itemSKUCode string

		if skuItem != nil {
			itemName = skuItem.Name
			itemPrice = skuItem.Price
			itemSKUCode = skuItem.SKU
		} else {
			// If not found in SKU Master, check if it is defined in Inventory Formula
			formula, err := u.formulaRepo.FindByCode(ctx, itemInput.SKU)
			if err != nil {
				return nil, err
			}
			if formula == nil || !formula.IsActive {
				return nil, appErrors.ErrSKUNotFound
			}
			itemName = formula.Name
			itemSKUCode = formula.Code
		}

		price := itemInput.Price
		if price <= 0 {
			price = itemPrice
		}

		subtotal := price * float64(itemInput.Quantity)
		totalAmount += subtotal

		items = append(items, domainOrder.OrderItem{
			SKU:      itemSKUCode,
			Name:     itemName,
			Price:    price,
			Quantity: itemInput.Quantity,
			Subtotal: subtotal,
		})
	}

	// FULL-17: the VAT-inclusive total is finalized here, in the same place
	// line subtotals are computed. Callers must not adjust it afterwards.
	if in.VATIncluded {
		totalAmount = totalAmount * 1.07
	}

	order := &domainOrder.Order{
		OrderNo:      orderNo,
		CustomerID:   in.CustomerID,
		CustomerName: in.CustomerName,
		Channel:      in.Channel,
		Status:       domainOrder.StatusPending,
		TotalAmount:  totalAmount,
		Note:         in.Note,
		Items:        items,
	}

	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		if err := u.orderRepo.Create(txCtx, order); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return order, nil
}

func (u *orderUsecase) GetByID(ctx context.Context, id uint) (*domainOrder.Order, error) {
	o, err := u.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, appErrors.ErrNotFound
	}
	return o, nil
}

func (u *orderUsecase) List(ctx context.Context, query domainOrder.Query) ([]domainOrder.Order, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	return u.orderRepo.FindAll(ctx, query)
}

// ShipOrder executes transaction: locks the order row, resolves bundle items,
// verifies stock under the same lock, deducts stock, records movements,
// updates status (FULL-07: no read-then-act race between concurrent ships).
func (u *orderUsecase) ShipOrder(ctx context.Context, id uint, warehouseID uint) (*domainOrder.Order, error) {
	if warehouseID == 0 {
		warehouseID = 1
	}

	var shipped *domainOrder.Order

	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		// Lock the order row first; all state checks happen against the
		// locked snapshot so a concurrent ship/cancel must wait here.
		orderItem, err := u.orderRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if orderItem == nil {
			return appErrors.ErrNotFound
		}

		if orderItem.Status == domainOrder.StatusShipped {
			return appErrors.NewAppError("ALREADY_SHIPPED", "Order has already been shipped", 400)
		}
		if orderItem.Status == domainOrder.StatusCancelled {
			return appErrors.NewAppError("ORDER_CANCELLED", "Cannot ship a cancelled order", 400)
		}

		totalCOGS := 0.0

		// Step 1: Resolve all physical SKU quantities to deduct across all order lines using shared resolver
		resolver := usecaseStock.NewDeductionResolver(u.formulaRepo, u.skuRepo)
		itemsToResolve := make([]usecaseStock.ItemToResolve, len(orderItem.Items))
		for i, line := range orderItem.Items {
			itemsToResolve[i] = usecaseStock.ItemToResolve{
				SKU:      line.SKU,
				Quantity: line.Quantity,
				Price:    line.Price,
			}
		}

		resolvedItems, err := resolver.ResolveDeductionItems(txCtx, itemsToResolve, "ORDER", orderItem.OrderNo)
		if err != nil {
			return err
		}

		// Aggregate resolved items by SKUCode
		type AggregatedItem struct {
			SKUCode           string
			Quantity          int
			SourceFormulaCode string
			RefType           string
			Note              string
		}
		aggMap := make(map[string]*AggregatedItem)
		for _, ri := range resolvedItems {
			if entry, exists := aggMap[ri.SKUCode]; exists {
				entry.Quantity += ri.Quantity
			} else {
				aggMap[ri.SKUCode] = &AggregatedItem{
					SKUCode:           ri.SKUCode,
					Quantity:          ri.Quantity,
					SourceFormulaCode: ri.SourceFormulaCode,
					RefType:           ri.RefType,
					Note:              ri.Note,
				}
			}
		}

		// Step 2: Validate stock under FOR UPDATE locks and compute COGS
		type ValidatedShipStock struct {
			Target *AggregatedItem
			SKU    *domainSKU.SKU
			Stock  *domainStock.Stock
			Lots   []domainStock.StockLot
		}
		var toShip []ValidatedShipStock

		for _, target := range aggMap {
			skuEntity, err := u.skuRepo.FindBySKU(txCtx, target.SKUCode)
			if err != nil || skuEntity == nil {
				return appErrors.ErrSKUNotFound
			}

			if skuEntity.CostPrice > 0 {
				totalCOGS += skuEntity.CostPrice * float64(target.Quantity)
			}

			stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, skuEntity.ID, warehouseID)
			if err != nil {
				return err
			}
			avail := 0
			if stk != nil {
				avail = stk.Quantity
			}
			if stk == nil || avail < target.Quantity {
				return appErrors.NewAppError("INSUFFICIENT_STOCK", fmt.Sprintf("Stock %s ไม่พอ: ต้องการ %d, คงเหลือ %d", target.SKUCode, target.Quantity, avail), 409)
			}

			// Load lots with FEFO lock
			lots, err := u.stockRepo.GetAvailableLotsForUpdate(txCtx, skuEntity.ID, warehouseID)
			if err != nil {
				return err
			}

			toShip = append(toShip, ValidatedShipStock{
				Target: target,
				SKU:    skuEntity,
				Stock:  stk,
				Lots:   lots,
			})
		}

		// Step 3: Deduct stock, deduct from FEFO lots, and record movements with lot/channel/formula metadata
		for _, vs := range toShip {
			updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, vs.SKU.ID, warehouseID, -vs.Target.Quantity)
			if err != nil {
				return err
			}

			// Release reservation if present
			if vs.Stock != nil && vs.Stock.ReservedQty > 0 {
				relQty := vs.Target.Quantity
				if relQty > vs.Stock.ReservedQty {
					relQty = vs.Stock.ReservedQty
				}
				_, _ = u.stockRepo.ReleaseStock(txCtx, vs.SKU.ID, warehouseID, relQty)
			}

			// FEFO Lot Allocation: deduct from earliest expiring lots
			needed := vs.Target.Quantity
			currentStockQty := vs.Stock.Quantity
			for _, lot := range vs.Lots {
				if needed <= 0 {
					break
				}
				lotAvail := lot.Quantity - lot.ReservedQty
				if lotAvail <= 0 {
					continue
				}
				deductQty := lotAvail
				if deductQty > needed {
					deductQty = needed
				}

				if _, err := u.stockRepo.DeductLotQuantity(txCtx, lot.ID, deductQty); err != nil {
					return err
				}

				movement := &domainStock.StockMovement{
					SKUID:             vs.SKU.ID,
					SKUCode:           vs.SKU.SKU,
					WarehouseID:       warehouseID,
					StockLotID:        &lot.ID,
					SourceFormulaCode: vs.Target.SourceFormulaCode,
					Channel:           "ORDER",
					Type:              domainStock.MovementOut,
					Quantity:          deductQty,
					BeforeQty:         currentStockQty,
					AfterQty:          currentStockQty - deductQty,
					ReferenceType:     vs.Target.RefType,
					ReferenceID:       orderItem.OrderNo,
					Note:              fmt.Sprintf("%s (Lot: %s)", vs.Target.Note, lot.LotNumber),
				}
				if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
					return err
				}
				currentStockQty -= deductQty
				needed -= deductQty
			}

			// If remaining needed > 0 (e.g. legacy stock without lot records), record remaining movement
			if needed > 0 {
				movement := &domainStock.StockMovement{
					SKUID:             vs.SKU.ID,
					SKUCode:           vs.SKU.SKU,
					WarehouseID:       warehouseID,
					SourceFormulaCode: vs.Target.SourceFormulaCode,
					Channel:           "ORDER",
					Type:              domainStock.MovementOut,
					Quantity:          needed,
					BeforeQty:         currentStockQty,
					AfterQty:          updatedStk.Quantity,
					ReferenceType:     vs.Target.RefType,
					ReferenceID:       orderItem.OrderNo,
					Note:              vs.Target.Note,
				}
				if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
					return err
				}
			}
		}

		// Update order status to SHIPPED inside the same transaction
		if err := u.orderRepo.UpdateStatus(txCtx, orderItem.ID, domainOrder.StatusShipped); err != nil {
			return err
		}

		// Auto-Post Journal Entry for Cost of Goods Sold (Dr. 5000 COGS / Cr. 1300 Inventory)
		// when finance repo is available and total COGS is positive.
		if u.financeRepo != nil && totalCOGS > 0 {
			accCOGS, _ := u.financeRepo.GetAccountByCode(txCtx, "5000")
			accInventory, _ := u.financeRepo.GetAccountByCode(txCtx, "1300")

			var cogsAccountID uint
			cogsAccountName := "ต้นทุนขาย (Cost of Goods Sold)"
			if accCOGS != nil {
				cogsAccountID = accCOGS.ID
				if accCOGS.Name != "" {
					cogsAccountName = accCOGS.Name
				}
			}

			var invAccountID uint
			invAccountName := "สินค้าคงเหลือ (Inventory)"
			if accInventory != nil {
				invAccountID = accInventory.ID
				if accInventory.Name != "" {
					invAccountName = accInventory.Name
				}
			}

			jeCode := fmt.Sprintf("JE-%s-%04d", time.Now().Format("2006/01/02"), time.Now().UnixNano()%10000)
			journal := &domainFinance.JournalEntry{
				Code:        jeCode,
				Date:        time.Now().Format("2006-01-02"),
				SourceType:  "sales_delivery",
				SourceID:    orderItem.ID,
				SourceRef:   orderItem.OrderNo,
				Description: fmt.Sprintf("ต้นทุนขายจากการจัดส่งสินค้าคำสั่งซื้อ %s", orderItem.OrderNo),
				Status:      domainFinance.JournalStatusPosted,
				CreatedBy:   "System (ShipOrder)",
				PostedAt:    time.Now().UTC(),
				Lines: []domainFinance.JournalLine{
					{
						AccountID:   cogsAccountID,
						AccountCode: "5000",
						AccountName: cogsAccountName,
						Debit:       totalCOGS,
						Credit:      0,
						Channel:     orderItem.Channel,
					},
					{
						AccountID:   invAccountID,
						AccountCode: "1300",
						AccountName: invAccountName,
						Debit:       0,
						Credit:      totalCOGS,
						Channel:     orderItem.Channel,
					},
				},
			}

			existing, _ := u.financeRepo.FindJournalEntryBySource(txCtx, "sales_delivery", orderItem.ID)
			if existing == nil {
				_ = u.financeRepo.CreateJournalEntry(txCtx, journal)
			}
		}

		orderItem.Status = domainOrder.StatusShipped
		shipped = orderItem
		return nil
	})

	if err != nil {
		return nil, err
	}

	return shipped, nil
}

// CancelOrder transitions to CANCELLED under the order row lock, so a cancel
// racing a ship cannot both succeed (FULL-07).
func (u *orderUsecase) CancelOrder(ctx context.Context, id uint) (*domainOrder.Order, error) {
	var cancelled *domainOrder.Order

	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		o, err := u.orderRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if o == nil {
			return appErrors.ErrNotFound
		}
		if o.Status == domainOrder.StatusShipped {
			return appErrors.NewAppError("CANNOT_CANCEL", "Cannot cancel an already shipped order", 400)
		}
		if o.Status == domainOrder.StatusCancelled {
			cancelled = o
			return nil
		}

		warehouseID := uint(1)
		// Release reserved stock for all order items (resolving formulas or direct SKU)
		type ReleaseTarget struct {
			SKUCode string
			Qty     int
			RefType string
			Note    string
		}
		releaseMap := make(map[string]*ReleaseTarget)

		for _, line := range o.Items {
			lineSKU := strings.ToUpper(strings.TrimSpace(line.SKU))
			formula, err := u.formulaRepo.FindByCode(txCtx, lineSKU)
			if err != nil {
				continue
			}

			if formula != nil && formula.IsActive && len(formula.Items) > 0 {
				for _, fi := range formula.Items {
					compCode := strings.ToUpper(strings.TrimSpace(fi.ComponentSKU))
					qtyToRel := fi.Qty * line.Quantity
					if entry, exists := releaseMap[compCode]; exists {
						entry.Qty += qtyToRel
					} else {
						releaseMap[compCode] = &ReleaseTarget{
							SKUCode: compCode,
							Qty:     qtyToRel,
							RefType: "ORDER_CANCEL_FORMULA",
							Note:    fmt.Sprintf("Released reserved stock for formula %s from cancelled order %s", line.SKU, o.OrderNo),
						}
					}
				}
			} else {
				if entry, exists := releaseMap[lineSKU]; exists {
					entry.Qty += line.Quantity
				} else {
					releaseMap[lineSKU] = &ReleaseTarget{
						SKUCode: lineSKU,
						Qty:     line.Quantity,
						RefType: "ORDER_CANCEL",
						Note:    fmt.Sprintf("Released reserved stock from cancelled order %s", o.OrderNo),
					}
				}
			}
		}

		for _, target := range releaseMap {
			skuEntity, err := u.skuRepo.FindBySKU(txCtx, target.SKUCode)
			if err != nil || skuEntity == nil {
				continue
			}

			stk, _ := u.stockRepo.GetBySKUIDForUpdate(txCtx, skuEntity.ID, warehouseID)
			if stk != nil && stk.ReservedQty > 0 {
				qtyToRel := target.Qty
				if qtyToRel > stk.ReservedQty {
					qtyToRel = stk.ReservedQty
				}
				updatedStk, err := u.stockRepo.ReleaseStock(txCtx, skuEntity.ID, warehouseID, qtyToRel)
				if err == nil && updatedStk != nil {
					movement := &domainStock.StockMovement{
						SKUID:         skuEntity.ID,
						SKUCode:       skuEntity.SKU,
						WarehouseID:   warehouseID,
						Type:          domainStock.MovementRelease,
						Quantity:      qtyToRel,
						BeforeQty:     stk.Quantity,
						AfterQty:      stk.Quantity,
						ReferenceType: target.RefType,
						ReferenceID:   o.OrderNo,
						Note:          target.Note,
					}
					_ = u.stockRepo.CreateMovement(txCtx, movement)
				}
			}
		}

		if err := u.orderRepo.UpdateStatus(txCtx, id, domainOrder.StatusCancelled); err != nil {
			return err
		}
		o.Status = domainOrder.StatusCancelled
		cancelled = o
		return nil
	})

	if err != nil {
		return nil, err
	}
	return cancelled, nil
}

// SalesListItem is one workspace sales-order row plus the status of the
// invoice linked to it (rendered as the "invRef" column).
type SalesListItem struct {
	Order         domainOrder.Order
	InvoiceStatus string
}

// ListSales returns the workspace sales-order list with the same filters the
// screen exposes.
func (u *orderUsecase) ListSales(ctx context.Context, q domainOrder.SalesQuery) ([]SalesListItem, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 50
	}

	orders, total, err := u.orderRepo.FindAllForSales(ctx, q)
	if err != nil {
		return nil, 0, err
	}

	invoiceStatus := make(map[uint]string, len(orders))
	if u.db != nil && len(orders) > 0 {
		orderIDs := make([]uint, 0, len(orders))
		for _, o := range orders {
			orderIDs = append(orderIDs, o.ID)
		}
		var rows []struct {
			OrderID uint
			Status  string
		}
		if err := u.db.WithContext(ctx).Table("invoices").
			Select("order_id, status").
			Where("order_id IN ?", orderIDs).
			Scan(&rows).Error; err == nil {
			for _, row := range rows {
				invoiceStatus[row.OrderID] = row.Status
			}
		}
	}

	items := make([]SalesListItem, len(orders))
	for i, o := range orders {
		items[i] = SalesListItem{Order: o, InvoiceStatus: invoiceStatus[o.ID]}
	}
	return items, total, nil
}

// GetByRef resolves an order by numeric id or document number.
func (u *orderUsecase) GetByRef(ctx context.Context, ref string) (*domainOrder.Order, error) {
	return u.orderRepo.FindByRef(ctx, ref)
}

// UpdateStatusByRef applies the workspace status transition rules. The returned
// bool reports whether the order already had the requested status.
func (u *orderUsecase) UpdateStatusByRef(ctx context.Context, ref, status, note string) (*domainOrder.Order, bool, error) {
	ord, err := u.orderRepo.FindByRef(ctx, ref)
	if err != nil {
		return nil, false, err
	}
	if ord == nil {
		return nil, false, appErrors.NewAppError("ORDER_NOT_FOUND", "Sales order not found", 404)
	}

	target := strings.ToUpper(strings.TrimSpace(status))
	if target == "COMPLETED" {
		target = "SHIPPED"
	}

	targetStatus := domainOrder.Status(target)
	if !domainOrder.IsValidStatus(targetStatus) {
		return nil, false, appErrors.NewAppError("INVALID_ORDER_STATUS",
			fmt.Sprintf("Invalid order status: %s", status), 400)
	}

	// Idempotency: same status.
	if ord.Status == targetStatus {
		return ord, true, nil
	}

	if !domainOrder.CanTransition(ord.Status, targetStatus) {
		return nil, false, appErrors.NewAppError("INVALID_ORDER_TRANSITION",
			fmt.Sprintf("Cannot transition order from %s to %s", ord.Status, target), 400)
	}

	if targetStatus == domainOrder.StatusCancelled {
		cancelled, err := u.CancelOrder(ctx, ord.ID)
		if err != nil {
			return nil, false, err
		}
		return cancelled, false, nil
	}

	if targetStatus == domainOrder.StatusShipped {
		shipped, err := u.ShipOrder(ctx, ord.ID, 1)
		if err != nil {
			return nil, false, appErrors.NewAppError("ORDER_SHIP_FAILED",
				"ไม่สามารถตัดสต็อกได้: "+err.Error(), 400)
		}
		return shipped, false, nil
	}

	ord.Status = targetStatus
	if note != "" {
		ord.Note = note
	}
	if err := u.orderRepo.Update(ctx, ord); err != nil {
		return nil, false, appErrors.NewAppError("ORDER_UPDATE_FAILED", "Failed to update order status", 500)
	}
	return ord, false, nil
}