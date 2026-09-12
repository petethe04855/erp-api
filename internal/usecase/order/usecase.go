package order

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainFinance "chawy-erp-api/internal/domain/finance"
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
	db         *gorm.DB
	orderRepo  domainOrder.Repository
	skuRepo    domainSKU.Repository
	bundleRepo domainBundle.Repository
	stockRepo  domainStock.Repository
	txMgr      database.TxManager
}

func NewOrderUsecase(
	db *gorm.DB,
	orderRepo domainOrder.Repository,
	skuRepo domainSKU.Repository,
	bundleRepo domainBundle.Repository,
	stockRepo domainStock.Repository,
) Usecase {
	var txMgr database.TxManager
	if db != nil {
		txMgr = database.NewTxManager(db)
	}
	return &orderUsecase{
		db:         db,
		orderRepo:  orderRepo,
		skuRepo:    skuRepo,
		bundleRepo: bundleRepo,
		stockRepo:  stockRepo,
		txMgr:      txMgr,
	}
}

func NewOrderUsecaseWithTx(
	db *gorm.DB,
	orderRepo domainOrder.Repository,
	skuRepo domainSKU.Repository,
	bundleRepo domainBundle.Repository,
	stockRepo domainStock.Repository,
	txMgr database.TxManager,
) Usecase {
	return &orderUsecase{
		db:         db,
		orderRepo:  orderRepo,
		skuRepo:    skuRepo,
		bundleRepo: bundleRepo,
		stockRepo:  stockRepo,
		txMgr:      txMgr,
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
		if skuItem == nil {
			return nil, appErrors.ErrSKUNotFound
		}

		price := itemInput.Price
		if price <= 0 {
			price = skuItem.Price
		}

		subtotal := price * float64(itemInput.Quantity)
		totalAmount += subtotal

		items = append(items, domainOrder.OrderItem{
			SKU:      skuItem.SKU,
			Name:     skuItem.Name,
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

	warehouseID := uint(1)

	// Execute stock verification, reservation, and order creation in a single transaction
	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		for _, item := range order.Items {
			skuEntity, err := u.skuRepo.FindBySKU(txCtx, item.SKU)
			if err != nil || skuEntity == nil {
				return appErrors.ErrSKUNotFound
			}

			if skuEntity.IsBundle {
				bundleItems, err := u.bundleRepo.GetItemsByBundleSKU(txCtx, skuEntity.SKU)
				if err != nil {
					return err
				}
				if len(bundleItems) == 0 {
					return fmt.Errorf("cannot order bundle %s: no component items defined in formula", skuEntity.SKU)
				}

				for _, bi := range bundleItems {
					compSKU, err := u.skuRepo.FindBySKU(txCtx, bi.ComponentSKU)
					if err != nil || compSKU == nil {
						return fmt.Errorf("bundle component %s not found", bi.ComponentSKU)
					}

					qtyToReserve := bi.Quantity * item.Quantity
					stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, compSKU.ID, warehouseID)
					if err != nil {
						return err
					}
					if stk == nil || stk.AvailableQty < qtyToReserve {
						avail := 0
						if stk != nil {
							avail = stk.AvailableQty
						}
						return appErrors.NewAppError(
							"INSUFFICIENT_STOCK",
							fmt.Sprintf("Stock %s ไม่พอ: ต้องการ %d, พร้อมขาย %d", bi.ComponentSKU, qtyToReserve, avail),
							409,
						)
					}

					if _, err := u.stockRepo.ReserveStock(txCtx, compSKU.ID, warehouseID, qtyToReserve); err != nil {
						return err
					}

					movement := &domainStock.StockMovement{
						SKUID:         compSKU.ID,
						SKUCode:       compSKU.SKU,
						WarehouseID:   warehouseID,
						Type:          domainStock.MovementReserve,
						Quantity:      qtyToReserve,
						BeforeQty:     stk.Quantity,
						AfterQty:      stk.Quantity,
						ReferenceType: "ORDER_RESERVE",
						ReferenceID:   order.OrderNo,
						Note:          fmt.Sprintf("Reserved for bundle %s in order %s", item.SKU, order.OrderNo),
					}
					if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
						return err
					}
				}
			} else {
				stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, skuEntity.ID, warehouseID)
				if err != nil {
					return err
				}
				if stk == nil || stk.AvailableQty < item.Quantity {
					avail := 0
					if stk != nil {
						avail = stk.AvailableQty
					}
					return appErrors.NewAppError(
						"INSUFFICIENT_STOCK",
						fmt.Sprintf("Stock %s ไม่พอ: ต้องการ %d, พร้อมขาย %d", item.SKU, item.Quantity, avail),
						409,
					)
				}

				if _, err := u.stockRepo.ReserveStock(txCtx, skuEntity.ID, warehouseID, item.Quantity); err != nil {
					return err
				}

				movement := &domainStock.StockMovement{
					SKUID:         skuEntity.ID,
					SKUCode:       skuEntity.SKU,
					WarehouseID:   warehouseID,
					Type:          domainStock.MovementReserve,
					Quantity:      item.Quantity,
					BeforeQty:     stk.Quantity,
					AfterQty:      stk.Quantity,
					ReferenceType: "ORDER_RESERVE",
					ReferenceID:   order.OrderNo,
					Note:          fmt.Sprintf("Reserved for order %s", order.OrderNo),
				}
				if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
					return err
				}
			}
		}

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

		for _, line := range orderItem.Items {
			if line.Quantity <= 0 {
				return appErrors.NewAppError("INVALID_QUANTITY", fmt.Sprintf("Order line %s has invalid quantity", line.SKU), 400)
			}

			skuEntity, err := u.skuRepo.FindBySKU(txCtx, line.SKU)
			if err != nil || skuEntity == nil {
				return appErrors.ErrSKUNotFound
			}

			if skuEntity.IsBundle {
				// Bundle resolution: load bundle components
				bundleItems, err := u.bundleRepo.GetItemsByBundleSKU(txCtx, skuEntity.SKU)
				if err != nil {
					return err
				}
				if len(bundleItems) == 0 {
					return fmt.Errorf("cannot ship bundle %s: no component items defined in formula", skuEntity.SKU)
				}

				for _, bi := range bundleItems {
					compSKU, err := u.skuRepo.FindBySKU(txCtx, bi.ComponentSKU)
					if err != nil || compSKU == nil {
						return fmt.Errorf("bundle component %s not found", bi.ComponentSKU)
					}

					qtyToDeduct := bi.Quantity * line.Quantity
					if compSKU.CostPrice > 0 {
						totalCOGS += compSKU.CostPrice * float64(qtyToDeduct)
					}

					// Check availability under the stock row lock
					stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, compSKU.ID, warehouseID)
					if err != nil {
						return err
					}
					if stk == nil || stk.Quantity < qtyToDeduct {
						avail := 0
						if stk != nil {
							avail = stk.Quantity
						}
						return appErrors.NewAppError("INSUFFICIENT_STOCK", fmt.Sprintf("Stock %s ไม่พอ: ต้องการ %d, คงเหลือ %d", bi.ComponentSKU, qtyToDeduct, avail), 409)
					}

					updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, compSKU.ID, warehouseID, -qtyToDeduct)
					if err != nil {
						return err
					}
					// Also release the reservation for this order
					_, _ = u.stockRepo.ReleaseStock(txCtx, compSKU.ID, warehouseID, qtyToDeduct)

					movement := &domainStock.StockMovement{
						SKUID:         compSKU.ID,
						SKUCode:       compSKU.SKU,
						WarehouseID:   warehouseID,
						Type:          domainStock.MovementOut,
						Quantity:      qtyToDeduct,
						BeforeQty:     stk.Quantity,
						AfterQty:      updatedStk.Quantity,
						ReferenceType: "ORDER_BUNDLE",
						ReferenceID:   orderItem.OrderNo,
						Note:          fmt.Sprintf("Shipped for bundle %s in order %s", line.SKU, orderItem.OrderNo),
					}
					if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
						return err
					}
				}
			} else {
				if skuEntity.CostPrice > 0 {
					totalCOGS += skuEntity.CostPrice * float64(line.Quantity)
				}

				// Single SKU: check total quantity under the stock row lock
				stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, skuEntity.ID, warehouseID)
				if err != nil {
					return err
				}
				if stk == nil || stk.Quantity < line.Quantity {
					avail := 0
					if stk != nil {
						avail = stk.Quantity
					}
					return appErrors.NewAppError("INSUFFICIENT_STOCK", fmt.Sprintf("Stock %s ไม่พอ: ต้องการ %d, คงเหลือ %d", line.SKU, line.Quantity, avail), 409)
				}

				updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, skuEntity.ID, warehouseID, -line.Quantity)
				if err != nil {
					return err
				}
				// Also release the reservation for this order
				_, _ = u.stockRepo.ReleaseStock(txCtx, skuEntity.ID, warehouseID, line.Quantity)

				movement := &domainStock.StockMovement{
					SKUID:         skuEntity.ID,
					SKUCode:       skuEntity.SKU,
					WarehouseID:   warehouseID,
					Type:          domainStock.MovementOut,
					Quantity:      line.Quantity,
					BeforeQty:     stk.Quantity,
					AfterQty:      updatedStk.Quantity,
					ReferenceType: "ORDER",
					ReferenceID:   orderItem.OrderNo,
					Note:          fmt.Sprintf("Shipped for order %s", orderItem.OrderNo),
				}
				if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
					return err
				}

				// Deduct associated accessories for non-bundle SKU
				var accessories []domainSKU.SKUAccessory
				if err := u.db.WithContext(txCtx).Where("UPPER(sku) = ?", strings.ToUpper(skuEntity.SKU)).Find(&accessories).Error; err == nil && len(accessories) > 0 {
					for _, acc := range accessories {
						accSKU, err := u.skuRepo.FindBySKU(txCtx, acc.AccessorySKU)
						if err != nil || accSKU == nil {
							return fmt.Errorf("accessory %s not found for SKU %s", acc.AccessorySKU, skuEntity.SKU)
						}
						accQtyToDeduct := acc.Quantity * line.Quantity
						if accSKU.CostPrice > 0 {
							totalCOGS += accSKU.CostPrice * float64(accQtyToDeduct)
						}

						accStk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, accSKU.ID, warehouseID)
						if err != nil {
							return err
						}
						if accStk == nil || accStk.Quantity < accQtyToDeduct {
							avail := 0
							if accStk != nil {
								avail = accStk.Quantity
							}
							return appErrors.NewAppError(
								"INSUFFICIENT_STOCK",
								fmt.Sprintf("Stock Accessory %s ไม่พอ: ต้องการ %d, คงเหลือ %d", acc.AccessorySKU, accQtyToDeduct, avail),
								409,
							)
						}
						updatedAccStk, err := u.stockRepo.UpdateQuantity(txCtx, accSKU.ID, warehouseID, -accQtyToDeduct)
						if err != nil {
							return err
						}
						accMovement := &domainStock.StockMovement{
							SKUID:         accSKU.ID,
							SKUCode:       accSKU.SKU,
							WarehouseID:   warehouseID,
							Type:          domainStock.MovementOut,
							Quantity:      accQtyToDeduct,
							BeforeQty:     accStk.Quantity,
							AfterQty:      updatedAccStk.Quantity,
							ReferenceType: "ORDER_ACCESSORY",
							ReferenceID:   orderItem.OrderNo,
							Note:          fmt.Sprintf("Shipped accessory %s for %s in order %s", acc.AccessorySKU, skuEntity.SKU, orderItem.OrderNo),
						}
						if err := u.stockRepo.CreateMovement(txCtx, accMovement); err != nil {
							return err
						}
					}
				}
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
		// Release reserved stock for all order items
		for _, line := range o.Items {
			skuEntity, err := u.skuRepo.FindBySKU(txCtx, line.SKU)
			if err != nil || skuEntity == nil {
				continue
			}

			if skuEntity.IsBundle {
				bundleItems, err := u.bundleRepo.GetItemsByBundleSKU(txCtx, skuEntity.SKU)
				if err != nil {
					continue
				}
				for _, bi := range bundleItems {
					compSKU, err := u.skuRepo.FindBySKU(txCtx, bi.ComponentSKU)
					if err != nil || compSKU == nil {
						continue
					}
					qtyToRelease := bi.Quantity * line.Quantity
					stk, err := u.stockRepo.ReleaseStock(txCtx, compSKU.ID, warehouseID, qtyToRelease)
					if err != nil {
						return err
					}
					movement := &domainStock.StockMovement{
						SKUID:         compSKU.ID,
						SKUCode:       compSKU.SKU,
						WarehouseID:   warehouseID,
						Type:          domainStock.MovementRelease,
						Quantity:      qtyToRelease,
						BeforeQty:     stk.Quantity,
						AfterQty:      stk.Quantity,
						ReferenceType: "ORDER_CANCEL",
						ReferenceID:   o.OrderNo,
						Note:          fmt.Sprintf("Released reserved stock for bundle %s from cancelled order %s", line.SKU, o.OrderNo),
					}
					_ = u.stockRepo.CreateMovement(txCtx, movement)
				}
			} else {
				stk, err := u.stockRepo.ReleaseStock(txCtx, skuEntity.ID, warehouseID, line.Quantity)
				if err != nil {
					return err
				}
				movement := &domainStock.StockMovement{
					SKUID:         skuEntity.ID,
					SKUCode:       skuEntity.SKU,
					WarehouseID:   warehouseID,
					Type:          domainStock.MovementRelease,
					Quantity:      line.Quantity,
					BeforeQty:     stk.Quantity,
					AfterQty:      stk.Quantity,
					ReferenceType: "ORDER_CANCEL",
					ReferenceID:   o.OrderNo,
					Note:          fmt.Sprintf("Released reserved stock from cancelled order %s", o.OrderNo),
				}
				_ = u.stockRepo.CreateMovement(txCtx, movement)
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
