package sku

import (
	"context"

	domainSKU "chawy-erp-api/internal/domain/sku"
	appErrors "chawy-erp-api/pkg/errors"
)

type CreateInput struct {
	SKU       string
	Name      string
	Barcode   string
	Category  string
	Price     float64
	CostPrice float64
	IsBundle  bool
	Image     string
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

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*domainSKU.SKU, error)
	GetByID(ctx context.Context, id uint) (*domainSKU.SKU, error)
	GetBySKU(ctx context.Context, skuCode string) (*domainSKU.SKU, error)
	List(ctx context.Context, query domainSKU.Query) ([]domainSKU.SKU, int64, error)
	Update(ctx context.Context, id uint, input UpdateInput) (*domainSKU.SKU, error)
	Delete(ctx context.Context, id uint) error
}

type skuUsecase struct {
	repo domainSKU.Repository
}

func NewSKUUsecase(repo domainSKU.Repository) Usecase {
	return &skuUsecase{repo: repo}
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
