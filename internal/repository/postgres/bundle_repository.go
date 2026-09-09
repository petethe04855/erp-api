package postgres

import (
	"context"

	"chawy-erp-api/internal/domain/bundle"

	"gorm.io/gorm"
)

type BundleRepository struct {
	db *gorm.DB
}

func NewBundleRepository(db *gorm.DB) bundle.Repository {
	return &BundleRepository{db: db}
}

func (r *BundleRepository) GetItemsByBundleSKU(ctx context.Context, bundleSKU string) ([]bundle.BundleItem, error) {
	var items []bundle.BundleItem
	err := r.db.WithContext(ctx).Where("bundle_sku = ?", bundleSKU).Find(&items).Error
	return items, err
}

func (r *BundleRepository) SaveItems(ctx context.Context, bundleSKU string, items []bundle.BundleItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing components
		if err := tx.Where("bundle_sku = ?", bundleSKU).Delete(&bundle.BundleItem{}).Error; err != nil {
			return err
		}

		if len(items) > 0 {
			for i := range items {
				items[i].BundleSKU = bundleSKU
			}
			return tx.Create(&items).Error
		}
		return nil
	})
}

func (r *BundleRepository) DeleteItem(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&bundle.BundleItem{}, id).Error
}
