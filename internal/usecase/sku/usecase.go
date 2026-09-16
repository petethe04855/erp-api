package sku

import (
	"context"
	"time"

	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	appErrors "chawy-erp-api/pkg/errors"
)

type CreateInput struct {
	SKU             string
	Name            string
	Barcode         string
	Category        string
	Price           float64
	CostPrice       float64
	IsBundle        bool
	Image           string
	InitialQuantity int
}

// UpdateInput uses pointer fields so an update only touches what the caller
// actually sent — omitted fields keep their stored values (FULL-11).
type UpdateInput struct {
	Name      *string
	Barcode   *string
	Category  *string
	Price     *float64
	CostPrice *float64
	IsBundle  *bool
	Image     *string
	Status    *string
}

type SKUWithStats struct {
	SKU            domainSKU.SKU
	LastReceivedAt *time.Time
	ReceiptCount   int
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*domainSKU.SKU, error)
	GetByID(ctx context.Context, id uint) (*domainSKU.SKU, error)
	GetBySKU(ctx context.Context, skuCode string) (*domainSKU.SKU, error)
	List(ctx context.Context, query domainSKU.Query) ([]domainSKU.SKU, int64, error)
	ListWithStats(ctx context.Context, query domainSKU.Query) ([]SKUWithStats, int64, error)
	GetReceiptHistory(ctx context.Context, query domainSKU.SKUReceiptQuery) ([]domainSKU.SKUReceiptItem, int64, error)
	Update(ctx context.Context, id uint, input UpdateInput) (*domainSKU.SKU, error)
	Delete(ctx context.Context, id uint) error
}

type skuUsecase struct {
	repo      domainSKU.Repository
	stockRepo domainStock.Repository
}

func NewSKUUsecase(repo domainSKU.Repository) Usecase {
	return &skuUsecase{repo: repo}
}

func NewSKUUsecaseWithStock(repo domainSKU.Repository, stockRepo domainStock.Repository) Usecase {
	return &skuUsecase{repo: repo, stockRepo: stockRepo}
}

func (u *skuUsecase) Create(ctx context.Context, input CreateInput) (*domainSKU.SKU, error) {
	exists, err := u.repo.ExistsBySKU(ctx, input.SKU)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, appErrors.ErrSKUAlreadyExists
	}

	item := &domainSKU.SKU{
		SKU:       input.SKU,
		Name:      input.Name,
		Barcode:   input.Barcode,
		Category:  input.Category,
		Price:     input.Price,
		CostPrice: input.CostPrice,
		IsBundle:  input.IsBundle,
		Image:     input.Image,
		Status:    "active",
	}

	if err := u.repo.Create(ctx, item); err != nil {
		return nil, err
	}

	// Initialize stock and record opening stock movement if stockRepo is configured
	if u.stockRepo != nil {
		initQty := input.InitialQuantity
		if initQty < 0 {
			initQty = 0
		}
		if initQty > 0 {
			_, _ = u.stockRepo.UpdateQuantity(ctx, item.ID, 1, initQty)
			_ = u.stockRepo.CreateMovement(ctx, &domainStock.StockMovement{
				SKUID:         item.ID,
				SKUCode:       item.SKU,
				WarehouseID:   1,
				Type:          domainStock.MovementIn,
				Quantity:      initQty,
				BeforeQty:     0,
				AfterQty:      initQty,
				ReferenceType: "INITIAL_STOCK",
				ReferenceID:   item.SKU,
				Note:          "Initial stock on SKU creation",
				CreatedAt:     time.Now(),
			})
		} else {
			// Create row with 0 quantity
			_, _ = u.stockRepo.UpdateQuantity(ctx, item.ID, 1, 0)
		}
	}

	return item, nil
}

func (u *skuUsecase) GetByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	item, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, appErrors.ErrSKUNotFound
	}
	return item, nil
}

func (u *skuUsecase) GetBySKU(ctx context.Context, skuCode string) (*domainSKU.SKU, error) {
	item, err := u.repo.FindBySKU(ctx, skuCode)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, appErrors.ErrSKUNotFound
	}
	return item, nil
}

func (u *skuUsecase) List(ctx context.Context, query domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	return u.repo.FindAll(ctx, query)
}

func (u *skuUsecase) ListWithStats(ctx context.Context, query domainSKU.Query) ([]SKUWithStats, int64, error) {
	items, total, err := u.List(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 {
		return []SKUWithStats{}, total, nil
	}

	skuIDs := make([]uint, len(items))
	for i, it := range items {
		skuIDs[i] = it.ID
	}

	statsMap, err := u.repo.GetReceiptStatsBatch(ctx, skuIDs)
	if err != nil {
		// Log or proceed without stats
		statsMap = make(map[uint]domainSKU.SKUBatchReceiptStat)
	}

	result := make([]SKUWithStats, len(items))
	for i, it := range items {
		st := statsMap[it.ID]
		result[i] = SKUWithStats{
			SKU:            it,
			LastReceivedAt: st.LastReceivedAt,
			ReceiptCount:   st.ReceiptCount,
		}
	}

	return result, total, nil
}

func (u *skuUsecase) GetReceiptHistory(ctx context.Context, query domainSKU.SKUReceiptQuery) ([]domainSKU.SKUReceiptItem, int64, error) {
	if query.SKUID == 0 {
		return nil, 0, appErrors.ErrSKUNotFound
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 10
	}
	// Verify SKU exists
	item, err := u.repo.FindByID(ctx, query.SKUID)
	if err != nil {
		return nil, 0, err
	}
	if item == nil {
		return nil, 0, appErrors.ErrSKUNotFound
	}

	return u.repo.GetReceiptHistory(ctx, query)
}


func (u *skuUsecase) Update(ctx context.Context, id uint, input UpdateInput) (*domainSKU.SKU, error) {
	item, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, appErrors.ErrSKUNotFound
	}

	if input.Name != nil && *input.Name != "" {
		item.Name = *input.Name
	}
	if input.Barcode != nil {
		item.Barcode = *input.Barcode
	}
	if input.Category != nil && *input.Category != "" {
		item.Category = *input.Category
	}
	if input.Price != nil && *input.Price > 0 {
		item.Price = *input.Price
	}
	if input.CostPrice != nil && *input.CostPrice >= 0 {
		item.CostPrice = *input.CostPrice
	}
	if input.IsBundle != nil {
		item.IsBundle = *input.IsBundle
	}
	if input.Image != nil {
		item.Image = *input.Image
	}
	if input.Status != nil && *input.Status != "" {
		item.Status = *input.Status
	}

	if err := u.repo.Update(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (u *skuUsecase) Delete(ctx context.Context, id uint) error {
	exists, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if exists == nil {
		return appErrors.ErrSKUNotFound
	}
	return u.repo.Delete(ctx, id)
}
