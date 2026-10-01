package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chawy-erp-api/internal/domain/shopee"
	pkgDatabase "chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ShopeeIncomeRepository struct {
	db *gorm.DB
}

func NewShopeeIncomeRepository(db *gorm.DB) shopee.IncomeRepository {
	return &ShopeeIncomeRepository{db: db}
}

func (r *ShopeeIncomeRepository) handle(ctx context.Context) *gorm.DB {
	return pkgDatabase.GetDBFromContext(ctx, r.db)
}

func (r *ShopeeIncomeRepository) BulkInsert(ctx context.Context, incomes []shopee.ShopeeIncome) (int, int, error) {
	if len(incomes) == 0 {
		return 0, 0, nil
	}

	inserted := 0
	skipped := 0

	err := r.handle(ctx).Transaction(func(tx *gorm.DB) error {
		for _, inc := range incomes {
			res := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "row_hash"}},
				DoNothing: true,
			}).Create(&inc)

			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				inserted++
			} else {
				skipped++
			}
		}
		return nil
	})

	if err != nil {
		return 0, 0, err
	}

	return inserted, skipped, nil
}

func (r *ShopeeIncomeRepository) FindAll(ctx context.Context, filter shopee.IncomeFilter) ([]shopee.ShopeeIncome, int64, error) {
	db := r.handle(ctx).Model(&shopee.ShopeeIncome{})

	if filter.Search != "" {
		s := "%" + strings.TrimSpace(filter.Search) + "%"
		db = db.Where("order_id ILIKE ?", s)
	}

	if filter.StartDate != nil {
		db = db.Where("transfer_date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		db = db.Where("transfer_date <= ?", *filter.EndDate)
	}

	if filter.Status == "matched" {
		db = db.Where("order_id IN (SELECT id FROM shopee_orders)")
	} else if filter.Status == "unmatched" {
		db = db.Where("order_id NOT IN (SELECT id FROM shopee_orders)")
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

	var incomes []shopee.ShopeeIncome
	err := db.Order("transfer_date DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&incomes).Error

	return incomes, total, err
}

func (r *ShopeeIncomeRepository) FindByMonth(ctx context.Context, year int, month int) ([]shopee.ShopeeIncome, error) {
	startDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)

	var list []shopee.ShopeeIncome
	err := r.handle(ctx).
		Where("transfer_date >= ? AND transfer_date < ?", startDate, endDate).
		Order("transfer_date ASC, order_id ASC").
		Find(&list).Error

	return list, err
}

func (r *ShopeeIncomeRepository) Delete(ctx context.Context, id uint) error {
	return r.handle(ctx).Delete(&shopee.ShopeeIncome{}, id).Error
}

func (r *ShopeeIncomeRepository) GetMatchedOrderIDs(ctx context.Context, orderIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(orderIDs) == 0 {
		return result, nil
	}

	var existingIDs []string
	err := r.handle(ctx).
		Model(&shopee.ShopeeOrder{}).
		Where("id IN ?", orderIDs).
		Pluck("id", &existingIDs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to check matched orders: %w", err)
	}

	for _, id := range existingIDs {
		result[id] = true
	}

	return result, nil
}
