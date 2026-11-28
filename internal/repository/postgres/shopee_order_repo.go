package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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
					// Identify the line by what the export says and users never edit:
					// the SKU as imported (sku may have been corrected since) and the
					// line total. Matching on qty would re-insert every line whose
					// quantity was stored wrongly by the old parser.
					var existingItem shopee.ShopeeOrderItem
					err := tx.Where("order_id = ? AND (original_sku = ? OR (original_sku = '' AND sku = ?)) AND sale_price = ?",
						ord.ID, item.SKU, item.SKU, item.SalePrice).First(&existingItem).Error
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

	prov := filter.Province
	if prov == "" {
		prov = filter.Provider
	}
	if prov != "" {
		p := "%" + strings.TrimSpace(prov) + "%"
		db = db.Where("province ILIKE ?", p)
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

func (r *ShopeeOrderRepository) GetDistinctProvinces(ctx context.Context) ([]string, error) {
	var provinces []string
	err := r.handle(ctx).Model(&shopee.ShopeeOrder{}).
		Where("province IS NOT NULL AND TRIM(province) != ''").
		Distinct("province").
		Order("province ASC").
		Pluck("province", &provinces).Error
	if err != nil {
		return nil, err
	}
	return provinces, nil
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

// ApplyRepairs rewrites the fields a repair found wrong, all or nothing. Each
// update is guarded by the old value, so a row edited after the preview makes
// the whole batch fail instead of being overwritten.
func (r *ShopeeOrderRepository) ApplyRepairs(ctx context.Context, repairs []shopee.OrderRepair, batchID, actor, sourceFile string) error {
	if len(repairs) == 0 {
		return nil
	}
	return r.handle(ctx).Transaction(func(tx *gorm.DB) error {
		var logs []shopee.ShopeeOrderRepair
		addLog := func(orderID string, itemID *uint, field, oldV, newV string) {
			logs = append(logs, shopee.ShopeeOrderRepair{
				BatchID: batchID, OrderID: orderID, ItemID: itemID, Field: field,
				OldValue: oldV, NewValue: newV, Actor: actor, SourceFile: sourceFile,
			})
		}
		guard := func(res *gorm.DB, what string) error {
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return fmt.Errorf("%s changed since the preview; nothing was saved, preview the file again", what)
			}
			return nil
		}
		const ts = "2006-01-02T15:04:05Z07:00"

		for _, rep := range repairs {
			if rep.DateChanged {
				res := tx.Model(&shopee.ShopeeOrder{}).
					Where("id = ? AND order_date = ?", rep.OrderID, rep.OldOrderDate).
					Update("order_date", rep.NewOrderDate)
				if err := guard(res, "order "+rep.OrderID); err != nil {
					return err
				}
				addLog(rep.OrderID, nil, "order_date", rep.OldOrderDate.Format(ts), rep.NewOrderDate.Format(ts))
			}
			for _, it := range rep.Items {
				itemID := it.ItemID
				if it.OldQty != it.NewQty {
					res := tx.Model(&shopee.ShopeeOrderItem{}).
						Where("id = ? AND order_id = ? AND qty = ?", it.ItemID, rep.OrderID, it.OldQty).
						Update("qty", it.NewQty)
					if err := guard(res, fmt.Sprintf("item %d of order %s", it.ItemID, rep.OrderID)); err != nil {
						return err
					}
					addLog(rep.OrderID, &itemID, "qty", strconv.Itoa(it.OldQty), strconv.Itoa(it.NewQty))
				}
				if it.OldVariation != it.NewVariation {
					res := tx.Model(&shopee.ShopeeOrderItem{}).
						Where("id = ? AND order_id = ? AND variation_name = ?", it.ItemID, rep.OrderID, it.OldVariation).
						Update("variation_name", it.NewVariation)
					if err := guard(res, fmt.Sprintf("item %d of order %s", it.ItemID, rep.OrderID)); err != nil {
						return err
					}
					addLog(rep.OrderID, &itemID, "variation_name", it.OldVariation, it.NewVariation)
				}
			}
		}
		if len(logs) == 0 {
			return nil
		}
		return tx.CreateInBatches(logs, 200).Error
	})
}

// ListInsightLines implements shopee.InsightRepository.
func (r *ShopeeOrderRepository) ListInsightLines(ctx context.Context, until time.Time) ([]shopee.InsightLine, error) {
	var lines []shopee.InsightLine
	err := r.handle(ctx).
		Table("shopee_order_items AS i").
		Select(`o.id AS order_id, o.order_date, o.buyer_username, i.product_name, i.variation_name, i.sku, i.qty, i.sale_price`).
		Joins("JOIN shopee_orders AS o ON o.id = i.order_id").
		Where("o.status = ? AND o.order_date < ?", "COMPLETED", until).
		Order("o.order_date, o.id, i.id").
		Scan(&lines).Error
	return lines, err
}

// NewShopeeInsightRepository returns the order repository as an InsightRepository.
func NewShopeeInsightRepository(db *gorm.DB) shopee.InsightRepository {
	return &ShopeeOrderRepository{db: db}
}
