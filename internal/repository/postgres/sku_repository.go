package postgres

import (
	"context"
	"errors"

	"chawy-erp-api/internal/domain/sku"
	pkgDatabase "chawy-erp-api/pkg/database"

	"gorm.io/gorm"
)

type SKURepository struct {
	db *gorm.DB
}

func NewSKURepository(db *gorm.DB) sku.Repository {
	return &SKURepository{db: db}
}

func (r *SKURepository) handle(ctx context.Context) *gorm.DB {
	return pkgDatabase.GetDBFromContext(ctx, r.db)
}

func (r *SKURepository) Create(ctx context.Context, item *sku.SKU) error {
	return r.handle(ctx).Create(item).Error
}

func (r *SKURepository) FindByID(ctx context.Context, id uint) (*sku.SKU, error) {
	var item sku.SKU
	err := r.handle(ctx).First(&item, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *SKURepository) FindBySKU(ctx context.Context, skuCode string) (*sku.SKU, error) {
	var item sku.SKU
	err := r.handle(ctx).Where("sku = ?", skuCode).First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *SKURepository) FindAll(ctx context.Context, q sku.Query) ([]sku.SKU, int64, error) {
	var items []sku.SKU
	var total int64

	tx := r.handle(ctx).Model(&sku.SKU{})

	if q.Search != "" {
		searchPattern := "%" + q.Search + "%"
		tx = tx.Where("sku ILIKE ? OR name ILIKE ? OR barcode ILIKE ?", searchPattern, searchPattern, searchPattern)
	}

	if q.Category != "" {
		tx = tx.Where("category = ?", q.Category)
	}

	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (q.Page - 1) * q.Limit
	if offset < 0 {
		offset = 0
	}

	err := tx.Offset(offset).Limit(q.Limit).Order("id DESC").Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *SKURepository) Update(ctx context.Context, item *sku.SKU) error {
	return r.handle(ctx).Save(item).Error
}

func (r *SKURepository) Delete(ctx context.Context, id uint) error {
	return r.handle(ctx).Delete(&sku.SKU{}, id).Error
}

func (r *SKURepository) ExistsBySKU(ctx context.Context, skuCode string) (bool, error) {
	var count int64
	err := r.handle(ctx).Model(&sku.SKU{}).Where("sku = ?", skuCode).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
