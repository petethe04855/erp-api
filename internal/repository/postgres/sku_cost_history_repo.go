package postgres

import (
	"context"
	"errors"
	"time"

	"chawy-erp-api/internal/domain/sku"
	pkgDatabase "chawy-erp-api/pkg/database"

	"gorm.io/gorm"
)

type SKUCostHistoryRepository struct {
	db *gorm.DB
}

func NewSKUCostHistoryRepository(db *gorm.DB) sku.CostHistoryRepository {
	return &SKUCostHistoryRepository{db: db}
}

func (r *SKUCostHistoryRepository) handle(ctx context.Context) *gorm.DB {
	return pkgDatabase.GetDBFromContext(ctx, r.db)
}

func (r *SKUCostHistoryRepository) Create(ctx context.Context, item *sku.SKUCostHistory) error {
	return r.handle(ctx).Create(item).Error
}

func (r *SKUCostHistoryRepository) FindBySKU(ctx context.Context, skuCode string) ([]sku.SKUCostHistory, error) {
	var list []sku.SKUCostHistory
	err := r.handle(ctx).
		Where("sku = ?", skuCode).
		Order("effective_from DESC, id DESC").
		Find(&list).Error
	return list, err
}

func (r *SKUCostHistoryRepository) FindEffectiveCost(ctx context.Context, skuCode string, date time.Time) (float64, bool, error) {
	var item sku.SKUCostHistory
	targetDate := date.Format("2006-01-02")
	err := r.handle(ctx).
		Where("sku = ? AND effective_from <= ? AND (effective_to IS NULL OR effective_to >= ?)", skuCode, targetDate, targetDate).
		Order("effective_from DESC, id DESC").
		First(&item).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return item.CostPrice, true, nil
}

func (r *SKUCostHistoryRepository) Delete(ctx context.Context, id uint) error {
	return r.handle(ctx).Delete(&sku.SKUCostHistory{}, id).Error
}
