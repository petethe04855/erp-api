package stock

import (
	"context"
	"fmt"

	domainStock "chawy-erp-api/internal/domain/stock"
	"chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"
)

type AdjustInput struct {
	SKUID         uint
	SKUCode       string
	WarehouseID   uint
	Type          domainStock.MovementType
	Quantity      int
	ReferenceType string
	ReferenceID   string
	Note          string
}

type Usecase interface {
	GetStock(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error)
	ListStock(ctx context.Context, query domainStock.Query) ([]domainStock.Stock, int64, error)
	AdjustStock(ctx context.Context, input AdjustInput) (*domainStock.Stock, error)
	GetMovements(ctx context.Context, skuID uint, page, limit int) ([]domainStock.StockMovement, int64, error)
}

type stockUsecase struct {
	repo  domainStock.Repository
	txMgr database.TxManager
}

func NewStockUsecase(repo domainStock.Repository) Usecase {
	return &stockUsecase{repo: repo}
}

// NewStockUsecaseWithTx allows wiring a transaction manager so adjustments
// and their movement records commit atomically (FULL-08).
func NewStockUsecaseWithTx(repo domainStock.Repository, txMgr database.TxManager) Usecase {
	return &stockUsecase{repo: repo, txMgr: txMgr}
}

func (u *stockUsecase) GetStock(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error) {
	if warehouseID == 0 {
		warehouseID = 1
	}
	stock, err := u.repo.GetBySKUID(ctx, skuID, warehouseID)
	if err != nil {
		return nil, err
	}
	if stock == nil {
		return &domainStock.Stock{
			SKUID:        skuID,
			WarehouseID:  warehouseID,
			Quantity:     0,
			AvailableQty: 0,
		}, nil
	}
	return stock, nil
}

func (u *stockUsecase) ListStock(ctx context.Context, query domainStock.Query) ([]domainStock.Stock, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	return u.repo.FindAll(ctx, query)
}

func (u *stockUsecase) AdjustStock(ctx context.Context, input AdjustInput) (*domainStock.Stock, error) {
	if input.WarehouseID == 0 {
		input.WarehouseID = 1
	}
	if input.Quantity < 0 {
		return nil, appErrors.NewAppError("INVALID_QUANTITY", "Quantity cannot be negative", 400)
	}

	if u.txMgr != nil {
		var result *domainStock.Stock
		err := u.txMgr.Transaction(ctx, func(txCtx context.Context) error {
			stock, txErr := u.adjustInTx(txCtx, input)
			if txErr != nil {
				return txErr
			}
			result = stock
			return nil
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	return u.adjustInTx(ctx, input)
}

// adjustInTx performs the adjustment and movement write under one stock-row
// lock, refusing any outcome that would drop available below zero (FULL-08).
func (u *stockUsecase) adjustInTx(ctx context.Context, input AdjustInput) (*domainStock.Stock, error) {
	if input.WarehouseID == 0 {
		input.WarehouseID = 1
	}

	currentStock, err := u.repo.GetBySKUIDForUpdate(ctx, input.SKUID, input.WarehouseID)
	if err != nil {
		return nil, err
	}
	if currentStock == nil {
		currentStock = &domainStock.Stock{
			SKUID:       input.SKUID,
			WarehouseID: input.WarehouseID,
			Quantity:    0,
			ReservedQty: 0,
		}
	}

	var delta int
	switch input.Type {
	case domainStock.MovementOut:
		if currentStock.AvailableQty < input.Quantity {
			return nil, appErrors.ErrInsufficientStock
		}
		delta = -input.Quantity
	case domainStock.MovementIn:
		delta = input.Quantity
	case domainStock.MovementAdjust:
		// FULL-08: a physical count must never dip below what is already
		// reserved for open orders.
		newQty := input.Quantity
		if newQty < currentStock.ReservedQty {
			return nil, appErrors.NewAppError(
				"BELOW_RESERVED",
				fmt.Sprintf("Adjusted quantity (%d) cannot be lower than reserved quantity (%d)", newQty, currentStock.ReservedQty),
				400,
			)
		}
		delta = newQty - currentStock.Quantity
	default:
		return nil, appErrors.NewAppError("INVALID_MOVEMENT_TYPE", "Unsupported stock movement type", 400)
	}

	updatedStock, err := u.repo.UpdateQuantity(ctx, input.SKUID, input.WarehouseID, delta)
	if err != nil {
		return nil, err
	}

	movement := &domainStock.StockMovement{
		SKUID:         input.SKUID,
		SKUCode:       input.SKUCode,
		WarehouseID:   input.WarehouseID,
		Type:          input.Type,
		Quantity:      input.Quantity,
		BeforeQty:     currentStock.Quantity,
		AfterQty:      updatedStock.Quantity,
		ReferenceType: input.ReferenceType,
		ReferenceID:   input.ReferenceID,
		Note:          input.Note,
	}
	if err := u.repo.CreateMovement(ctx, movement); err != nil {
		return nil, err
	}

	return updatedStock, nil
}

func (u *stockUsecase) GetMovements(ctx context.Context, skuID uint, page, limit int) ([]domainStock.StockMovement, int64, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return u.repo.GetMovements(ctx, skuID, page, limit)
}
