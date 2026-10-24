package report

import (
	"context"
	"time"
)

type ManualRevenueRecord struct {
	InvoiceNo    string
	CustomerName string
	Amount       float64
	PaidAt       *time.Time
	CreatedAt    time.Time
}

type TikTokOrderRecord struct {
	ID     string
	Date   string
	Amount float64
}

type StockValuationItem struct {
	SKUCode   string
	SKUName   string
	Quantity  int
	CostPrice float64
	Category  string
}

type Repository interface {
	GetDashboardSummary(ctx context.Context) (*DashboardSummary, error)
	GetPaidManualInvoices(ctx context.Context, startTime, endTime string) ([]ManualRevenueRecord, error)
	GetCompletedTikTokOrders(ctx context.Context, startTime, endTime string) ([]TikTokOrderRecord, error)
	GetCOGS(ctx context.Context, startTime, endTime string) (float64, error)
	GetStockValuationItems(ctx context.Context) ([]StockValuationItem, error)
}
