package stock

import (
	"context"

	domainStock "chawy-erp-api/internal/domain/stock"
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
	repo domainStock.Repository
}

func NewStockUsecase(repo domainStock.Repository) Usecase {
	return &stockUsecase{repo: repo}
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

	currentStock, err := u.GetStock(ctx, input.SKUID, input.WarehouseID)
	if err != nil {
		return nil, err
	}

	delta := input.Quantity
	if input.Type == domainStock.MovementOut {
		if currentStock.AvailableQty < input.Quantity {
			return nil, appErrors.ErrInsufficientStock
		}
		delta = -input.Quantity
	} else if input.Type == domainStock.MovementAdjust {
		delta = input.Quantity - currentStock.Quantity
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
