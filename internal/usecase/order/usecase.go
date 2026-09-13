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
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
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
	txMgr       database.TxManager
}

func NewOrderUsecase(
	db *gorm.DB,
	orderRepo domainOrder.Repository,
	skuRepo domainSKU.Repository,
	bundleRepo domainBundle.Repository,
	formulaRepo domainFormula.Repository,
	stockRepo domainStock.Repository,
) Usecase {
	var txMgr database.TxManager
	if db != nil {
		txMgr = database.NewTxManager(db)
	}
	return &orderUsecase{
		db:          db,
		orderRepo:   orderRepo,
		skuRepo:     skuRepo,
		bundleRepo:  bundleRepo,
		formulaRepo: formulaRepo,
		stockRepo:   stockRepo,
		txMgr:       txMgr,
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
) Usecase {
	return &orderUsecase{
		db:          db,
		orderRepo:   orderRepo,
		skuRepo:     skuRepo,
		bundleRepo:  bundleRepo,
		formulaRepo: formulaRepo,
		stockRepo:   stockRepo,
		txMgr:       txMgr,
	}
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

	orderNo := fmt.Sprintf("SO-%s-%04d", time.Now().Format("20060102"), time.Now().UnixNano()%10000)

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

		// Step 1: Collect and aggregate all physical SKU quantities to deduct across all order lines
		type ShipTarget struct {
			SKUCode string
			Qty     int
			RefType string
			Note    string
		}
		shipMap := make(map[string]*ShipTarget)

		for _, line := range orderItem.Items {
			if line.Quantity <= 0 {
				return appErrors.NewAppError("INVALID_QUANTITY", fmt.Sprintf("Order line %s has invalid quantity", line.SKU), 400)
			}

			lineSKU := strings.ToUpper(strings.TrimSpace(line.SKU))
			formula, err := u.formulaRepo.FindByCode(txCtx, lineSKU)
			if err != nil {
				return err
			}

			if formula != nil && formula.IsActive && len(formula.Items) > 0 {
				// Explode components
				for _, fi := range formula.Items {
					compCode := strings.ToUpper(strings.TrimSpace(fi.ComponentSKU))
					neededQty := fi.Qty * line.Quantity
					if entry, exists := shipMap[compCode]; exists {
						entry.Qty += neededQty
					} else {
						shipMap[compCode] = &ShipTarget{
							SKUCode: compCode,
							Qty:     neededQty,
							RefType: "ORDER_FORMULA",
							Note:    fmt.Sprintf("Shipped for formula %s in order %s", line.SKU, orderItem.OrderNo),
						}
					}
				}
			} else {
				// No formula: direct SKU
				if entry, exists := shipMap[lineSKU]; exists {
					entry.Qty += line.Quantity
				} else {
					shipMap[lineSKU] = &ShipTarget{
						SKUCode: lineSKU,
						Qty:     line.Quantity,
						RefType: "ORDER",
						Note:    fmt.Sprintf("Shipped for order %s", orderItem.OrderNo),
					}
				}

				// Deduct associated accessories for non-formula SKU
				var accessories []domainSKU.SKUAccessory
				if err := u.db.WithContext(txCtx).Where("UPPER(sku) = ?", lineSKU).Find(&accessories).Error; err == nil && len(accessories) > 0 {
					for _, acc := range accessories {
						accCode := strings.ToUpper(strings.TrimSpace(acc.AccessorySKU))
						accQty := acc.Quantity * line.Quantity
						if entry, exists := shipMap[accCode]; exists {
							entry.Qty += accQty
						} else {
							shipMap[accCode] = &ShipTarget{
								SKUCode: accCode,
								Qty:     accQty,
								RefType: "ORDER_ACCESSORY",
								Note:    fmt.Sprintf("Shipped accessory %s for %s in order %s", acc.AccessorySKU, line.SKU, orderItem.OrderNo),
							}
						}
					}
				}
			}
		}

		// Step 2: Validate stock and calculate COGS under FOR UPDATE locks
		type ValidatedShipStock struct {
			Target *ShipTarget
			SKU    *domainSKU.SKU
			Stock  *domainStock.Stock
		}
		var toShip []ValidatedShipStock

		for _, target := range shipMap {
			skuEntity, err := u.skuRepo.FindBySKU(txCtx, target.SKUCode)
			if err != nil || skuEntity == nil {
				return appErrors.ErrSKUNotFound
			}

			if skuEntity.CostPrice > 0 {
				totalCOGS += skuEntity.CostPrice * float64(target.Qty)
			}

			stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, skuEntity.ID, warehouseID)
			if err != nil {
				return err
			}
			if stk == nil || stk.Quantity < target.Qty {
				avail := 0
				if stk != nil {
					avail = stk.Quantity
				}
				return appErrors.NewAppError("INSUFFICIENT_STOCK", fmt.Sprintf("Stock %s ไม่พอ: ต้องการ %d, คงเหลือ %d", target.SKUCode, target.Qty, avail), 409)
			}

			toShip = append(toShip, ValidatedShipStock{
				Target: target,
				SKU:    skuEntity,
				Stock:  stk,
			})
		}

		// Step 3: Deduct stock and release reservations (if any)
		for _, vs := range toShip {
			updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, vs.SKU.ID, warehouseID, -vs.Target.Qty)
			if err != nil {
				return err
			}
			// Safely release reservation if there was any historical reservation
			if vs.Stock != nil && vs.Stock.ReservedQty > 0 {
				relQty := vs.Target.Qty
				if relQty > vs.Stock.ReservedQty {
					relQty = vs.Stock.ReservedQty
				}
				_, _ = u.stockRepo.ReleaseStock(txCtx, vs.SKU.ID, warehouseID, relQty)
			}

			movement := &domainStock.StockMovement{
				SKUID:         vs.SKU.ID,
				SKUCode:       vs.SKU.SKU,
				WarehouseID:   warehouseID,
				Type:          domainStock.MovementOut,
				Quantity:      vs.Target.Qty,
				BeforeQty:     vs.Stock.Quantity,
				AfterQty:      updatedStk.Quantity,
				ReferenceType: vs.Target.RefType,
				ReferenceID:   orderItem.OrderNo,
				Note:          vs.Target.Note,
			}
			if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
				return err
			}
		}

		// Update order status to SHIPPED inside the same transaction
		if err := u.orderRepo.UpdateStatus(txCtx, orderItem.ID, domainOrder.StatusShipped); err != nil {
			return err
		}

		// Auto-Post Journal Entry for Cost of Goods Sold (Dr. 5000 COGS / Cr. 1300 Inventory)
		// when database transaction is available and total COGS is positive.
		if u.db != nil && totalCOGS > 0 {
			var accCOGS domainFinance.Account
			var accInventory domainFinance.Account
			dbTx := u.db.WithContext(txCtx)

			_ = dbTx.Where("code = ?", "5000").First(&accCOGS).Error
			_ = dbTx.Where("code = ?", "1300").First(&accInventory).Error

			cogsAccountID := accCOGS.ID
			cogsAccountName := accCOGS.Name
			if cogsAccountName == "" {
				cogsAccountName = "ต้นทุนขาย (Cost of Goods Sold)"
			}

			invAccountID := accInventory.ID
			invAccountName := accInventory.Name
			if invAccountName == "" {
				invAccountName = "สินค้าคงเหลือ (Inventory)"
			}

			jeCode := fmt.Sprintf("JE-%s-%04d", time.Now().Format("2006"), time.Now().UnixNano()%10000)
			journal := domainFinance.JournalEntry{
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

			var existingCount int64
			if err := dbTx.Model(&domainFinance.JournalEntry{}).
				Where("source_type = ? AND source_id = ?", "sales_delivery", orderItem.ID).
				Count(&existingCount).Error; err == nil && existingCount == 0 {
				_ = dbTx.Create(&journal).Error
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
