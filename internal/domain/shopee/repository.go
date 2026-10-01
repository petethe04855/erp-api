package shopee

import (
	"context"
	"time"
)

type OrderFilter struct {
	Search    string
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	Limit     int
}

type IncomeFilter struct {
	Search    string
	StartDate *time.Time
	EndDate   *time.Time
	Status    string // all, matched, unmatched
	Page      int
	Limit     int
}

type OrderRepository interface {
	BulkInsert(ctx context.Context, orders []ShopeeOrder) (inserted int, skipped int, err error)
	FindAll(ctx context.Context, filter OrderFilter) ([]ShopeeOrder, int64, error)
	FindByID(ctx context.Context, id string) (*ShopeeOrder, error)
	UpdateItemSKU(ctx context.Context, itemID uint, newSKU string, confirmed bool) error
	Delete(ctx context.Context, id string) error
	FindByOrderIDs(ctx context.Context, orderIDs []string) ([]ShopeeOrder, error)
}

type IncomeRepository interface {
	BulkInsert(ctx context.Context, incomes []ShopeeIncome) (inserted int, skipped int, err error)
	FindAll(ctx context.Context, filter IncomeFilter) ([]ShopeeIncome, int64, error)
	FindByMonth(ctx context.Context, year int, month int) ([]ShopeeIncome, error)
	Delete(ctx context.Context, id uint) error
	GetMatchedOrderIDs(ctx context.Context, orderIDs []string) (map[string]bool, error)
}
