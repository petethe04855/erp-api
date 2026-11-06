package postgres

import (
	"context"

	domainCustomer "chawy-erp-api/internal/domain/customer"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainReport "chawy-erp-api/internal/domain/report"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
)

type reportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) domainReport.Repository {
	return &reportRepository{db: db}
}

func (r *reportRepository) GetDashboardSummary(ctx context.Context) (*domainReport.DashboardSummary, error) {
	db := database.GetDBFromContext(ctx, r.db)
	var summary domainReport.DashboardSummary

	// Orders summary
	var totalOrders, pendingOrders, shippedOrders int64
	var totalSales float64

	if err := db.Model(&domainOrder.Order{}).Count(&totalOrders).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&domainOrder.Order{}).Where("status = ?", domainOrder.StatusPending).Count(&pendingOrders).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&domainOrder.Order{}).Where("status = ?", domainOrder.StatusShipped).Count(&shippedOrders).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&domainOrder.Order{}).Where("status = ?", domainOrder.StatusShipped).Select("COALESCE(SUM(total_amount), 0)").Scan(&totalSales).Error; err != nil {
		return nil, err
	}

	summary.TotalOrders = totalOrders
	summary.PendingOrders = pendingOrders
	summary.ShippedOrders = shippedOrders
	summary.TotalSales = totalSales

	// SKUs, Suppliers, Customers
	if err := db.Model(&domainSKU.SKU{}).Count(&summary.TotalSKUs).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&domainPurchasing.Supplier{}).Count(&summary.TotalSuppliers).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&domainCustomer.Customer{}).Count(&summary.TotalCustomers).Error; err != nil {
		return nil, err
	}

	// Invoices
	if err := db.Model(&domainInvoice.Invoice{}).Where("status = ?", domainInvoice.StatusUnpaid).Count(&summary.TotalUnpaidInvs).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&domainInvoice.Invoice{}).Where("status = ?", domainInvoice.StatusUnpaid).Select("COALESCE(SUM(amount), 0)").Scan(&summary.UnpaidInvsAmount).Error; err != nil {
		return nil, err
	}

	return &summary, nil
}

func (r *reportRepository) GetPaidManualInvoices(ctx context.Context, startTime, endTime string) ([]domainReport.ManualRevenueRecord, error) {
	db := database.GetDBFromContext(ctx, r.db)
	var records []domainReport.ManualRevenueRecord

	query := db.Table("invoices AS invoices").
		Select(`invoices.invoice_no,
			invoices.customer_name,
			invoices.amount,
			invoices.paid_at,
			invoices.created_at`).
		Joins("LEFT JOIN orders AS orders ON orders.id = invoices.order_id").
		Where("invoices.status = ?", domainInvoice.StatusPaid).
		Where("invoices.order_id IS NULL OR LOWER(TRIM(COALESCE(orders.channel, ''))) IN ?", []string{"", "direct", "manual"})

	if startTime != "" && endTime != "" {
		query = query.Where("COALESCE(invoices.paid_at, invoices.created_at) >= ? AND COALESCE(invoices.paid_at, invoices.created_at) < ?", startTime, endTime)
	}

	if err := query.Order("COALESCE(invoices.paid_at, invoices.created_at) DESC, invoices.id DESC").Scan(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}

func (r *reportRepository) GetCompletedTikTokOrders(ctx context.Context, startTime, endTime string) ([]domainReport.TikTokOrderRecord, error) {
	db := database.GetDBFromContext(ctx, r.db)
	var orders []domainTikTok.TiktokOrder

	query := db.Model(&domainTikTok.TiktokOrder{}).
		Where("UPPER(TRIM(status)) IN ?", []string{"COMPLETED", "DELIVERED", "IN_TRANSIT", "AWAITING_SHIPMENT"})

	if startTime != "" && endTime != "" {
		query = query.Where("date >= ? AND date < ?", startTime, endTime)
	}

	if err := query.Order("date DESC, id DESC").Find(&orders).Error; err != nil {
		return nil, err
	}

	records := make([]domainReport.TikTokOrderRecord, len(orders))
	for i, o := range orders {
		records[i] = domainReport.TikTokOrderRecord{
			ID:     o.ID,
			Date:   o.Date,
			Amount: o.Amount,
		}
	}
	return records, nil
}

func (r *reportRepository) GetCOGS(ctx context.Context, startTime, endTime string) (float64, error) {
	db := database.GetDBFromContext(ctx, r.db)
	var cogs float64

	cogsQuery := db.Table("order_items").
		Select("COALESCE(SUM(order_items.quantity * skus.cost_price), 0)").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("JOIN skus ON skus.sku = order_items.sku").
		Where("orders.status IN ?", []string{string(domainOrder.StatusShipped), "COMPLETED"})

	if startTime != "" && endTime != "" {
		cogsQuery = cogsQuery.Where("orders.created_at >= ? AND orders.created_at < ?", startTime, endTime)
	}

	err := cogsQuery.Scan(&cogs).Error
	return cogs, err
}

func (r *reportRepository) GetStockValuationItems(ctx context.Context) ([]domainReport.StockValuationItem, error) {
	db := database.GetDBFromContext(ctx, r.db)
	var results []domainReport.StockValuationItem

	err := db.Table("stocks").
		Select("skus.sku as sku_code, skus.name as sku_name, stocks.quantity, skus.cost_price, skus.category").
		Joins("JOIN skus ON skus.id = stocks.sku_id").
		Where("stocks.quantity > 0").
		Scan(&results).Error

	return results, err
}
