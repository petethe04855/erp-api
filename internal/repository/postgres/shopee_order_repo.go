package postgres

import (
	"context"
	"errors"
	"strings"

	"chawy-erp-api/internal/domain/shopee"
	pkgDatabase "chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ShopeeOrderRepository struct {
	db *gorm.DB
}

func NewShopeeOrderRepository(db *gorm.DB) shopee.OrderRepository {
	return &ShopeeOrderRepository{db: db}
}

func (r *ShopeeOrderRepository) handle(ctx context.Context) *gorm.DB {
	return pkgDatabase.GetDBFromContext(ctx, r.db)
}

func (r *ShopeeOrderRepository) BulkInsert(ctx context.Context, orders []shopee.ShopeeOrder) (int, int, error) {
	if len(orders) == 0 {
		return 0, 0, nil
	}

	inserted := 0
	skipped := 0

	err := r.handle(ctx).Transaction(func(tx *gorm.DB) error {
		for _, ord := range orders {
			var existing shopee.ShopeeOrder
			err := tx.Where("id = ?", ord.ID).First(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			if errors.Is(err, gorm.ErrRecordNotFound) {
				// New order
				if err := tx.Omit(clause.Associations).Create(&ord).Error; err != nil {
					return err
				}
				for _, item := range ord.Items {
					item.OrderID = ord.ID
					if err := tx.Create(&item).Error; err != nil {
						return err
					}
				}
				inserted++
			} else {
				// Existing order header - check if item already exists or if we should skip
				hasNewItems := false
				for _, item := range ord.Items {
					var existingItem shopee.ShopeeOrderItem
					err := tx.Where("order_id = ? AND sku = ? AND qty = ?", ord.ID, item.SKU, item.Qty).First(&existingItem).Error
					if errors.Is(err, gorm.ErrRecordNotFound) {
						item.OrderID = ord.ID
						if err := tx.Create(&item).Error; err != nil {
							return err
						}
						hasNewItems = true
					}
				}
				if hasNewItems {
					inserted++
				} else {
					skipped++
				}
			}
		}
		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	return inserted, skipped, nil
}

func (r *ShopeeOrderRepository) FindAll(ctx context.Context, filter shopee.OrderFilter) ([]shopee.ShopeeOrder, int64, error) {
	db := r.handle(ctx).Model(&shopee.ShopeeOrder{})

	if filter.Search != "" {
		s := "%" + strings.TrimSpace(filter.Search) + "%"
		db = db.Where("id ILIKE ? OR buyer_username ILIKE ? OR id IN (SELECT order_id FROM shopee_order_items WHERE sku ILIKE ? OR product_name ILIKE ?)", s, s, s, s)
	}

	if filter.StartDate != nil {
		db = db.Where("order_date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		db = db.Where("order_date <= ?", *filter.EndDate)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 50
	}
	offset := (page - 1) * limit

	var orders []shopee.ShopeeOrder
	err := db.Preload("Items").
		Order("order_date DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&orders).Error

	return orders, total, err
}

func (r *ShopeeOrderRepository) FindByID(ctx context.Context, id string) (*shopee.ShopeeOrder, error) {
	var ord shopee.ShopeeOrder
	err := r.handle(ctx).Preload("Items").Where("id = ?", id).First(&ord).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ord, nil
}

func (r *ShopeeOrderRepository) UpdateItemSKU(ctx context.Context, itemID uint, newSKU string, confirmed bool) error {
	var item shopee.ShopeeOrderItem
	if err := r.handle(ctx).First(&item, itemID).Error; err != nil {
		return err
	}

	updates := map[string]interface{}{
		"sku":           newSKU,
		"sku_confirmed": confirmed,
	}
	if item.OriginalSKU == "" {
		updates["original_sku"] = item.SKU
	}

	return r.handle(ctx).Model(&item).Updates(updates).Error
}

func (r *ShopeeOrderRepository) Delete(ctx context.Context, id string) error {
	return r.handle(ctx).Where("id = ?", id).Delete(&shopee.ShopeeOrder{}).Error
}

func (r *ShopeeOrderRepository) FindByOrderIDs(ctx context.Context, orderIDs []string) ([]shopee.ShopeeOrder, error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}
	var orders []shopee.ShopeeOrder
	err := r.handle(ctx).Preload("Items").Where("id IN ?", orderIDs).Find(&orders).Error
	return orders, err
}
