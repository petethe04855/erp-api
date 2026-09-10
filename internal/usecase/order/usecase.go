package order

import (
	"context"
	"fmt"
	"time"

	domainBundle "chawy-erp-api/internal/domain/bundle"
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
}

func NewOrderUsecase(
	db *gorm.DB,
	orderRepo domainOrder.Repository,
	skuRepo domainSKU.Repository,
	bundleRepo domainBundle.Repository,
	stockRepo domainStock.Repository,
) Usecase {
	return &orderUsecase{
		db:         db,
		orderRepo:  orderRepo,
		skuRepo:    skuRepo,
		bundleRepo: bundleRepo,
		stockRepo:  stockRepo,
	}
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

	if err := u.orderRepo.Create(ctx, order); err != nil {
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

	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := database.WithTxContext(ctx, tx)

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

					// Check availability under the stock row lock
					stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, compSKU.ID, warehouseID)
					if err != nil {
						return err
					}
					if stk == nil || stk.AvailableQty < qtyToDeduct {
						return fmt.Errorf("insufficient stock for component %s: required %d", bi.ComponentSKU, qtyToDeduct)
					}

					updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, compSKU.ID, warehouseID, -qtyToDeduct)
					if err != nil {
						return err
					}

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
				// Single SKU: check availability under the stock row lock
				stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, skuEntity.ID, warehouseID)
				if err != nil {
					return err
				}
				if stk == nil || stk.AvailableQty < line.Quantity {
					return fmt.Errorf("insufficient stock for SKU %s: required %d", line.SKU, line.Quantity)
				}

				updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, skuEntity.ID, warehouseID, -line.Quantity)
				if err != nil {
					return err
				}

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
			}
		}

		// Update order status to SHIPPED inside the same transaction
		if err := u.orderRepo.UpdateStatus(txCtx, orderItem.ID, domainOrder.StatusShipped); err != nil {
			return err
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

	err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := database.WithTxContext(ctx, tx)

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
