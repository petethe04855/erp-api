package shopee

import (
	"context"
	"time"
)

type OrderFilter struct {
	Search    string
	Province  string
	Provider  string
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
	GetDistinctProvinces(ctx context.Context) ([]string, error)
	UpdateItemSKU(ctx context.Context, itemID uint, newSKU string, confirmed bool) error
	Delete(ctx context.Context, id string) error
	FindByOrderIDs(ctx context.Context, orderIDs []string) ([]ShopeeOrder, error)
	// ApplyRepairs rewrites order dates, item quantities and variations in one
	// transaction, logging every change. A row that no longer holds the old value
	// (changed since the preview) fails the whole batch.
	ApplyRepairs(ctx context.Context, repairs []OrderRepair, batchID, actor, sourceFile string) error
}

type IncomeRepository interface {
	BulkInsert(ctx context.Context, incomes []ShopeeIncome) (inserted int, skipped int, err error)
	FindAll(ctx context.Context, filter IncomeFilter) ([]ShopeeIncome, int64, error)
	FindByMonth(ctx context.Context, year int, month int) ([]ShopeeIncome, error)
	Delete(ctx context.Context, id uint) error
	GetMatchedOrderIDs(ctx context.Context, orderIDs []string) (map[string]bool, error)
	FindByRowHashes(ctx context.Context, hashes []string) ([]ShopeeIncome, error)
}

// InsightLine is one completed Shopee order line with what the sales insights need.
type InsightLine struct {
	OrderID       string
	OrderDate     time.Time
	BuyerUsername string
	ProductName   string
	VariationName string
	SKU           string
	Qty           int
	SalePrice     float64
}

// InsightRepository reads order lines for the sales insights.
type InsightRepository interface {
	// ListInsightLines returns every completed order line ordered before `until`
	// (the whole history, so first purchases can be found), oldest first.
	ListInsightLines(ctx context.Context, until time.Time) ([]InsightLine, error)
}
