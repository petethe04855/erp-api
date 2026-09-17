package report

import (
	"context"
	"sort"
	"time"

	domainCustomer "chawy-erp-api/internal/domain/customer"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainReport "chawy-erp-api/internal/domain/report"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"

	"gorm.io/gorm"
)

type Usecase interface {
	GetDashboardSummary(ctx context.Context) (*domainReport.DashboardSummary, error)
	GetRevenueReport(ctx context.Context, month string) (*domainReport.RevenueReport, error)
	GetFinancialSummary(ctx context.Context, month string) (*domainReport.FinancialSummary, error)
	GetInventoryValuation(ctx context.Context) (*domainReport.InventoryValuation, error)
}

type reportUsecase struct {
	db *gorm.DB
}

func NewReportUsecase(db *gorm.DB) Usecase {
	return &reportUsecase{db: db}
}

func (u *reportUsecase) GetDashboardSummary(ctx context.Context) (*domainReport.DashboardSummary, error) {
	var summary domainReport.DashboardSummary

	// Orders summary
	var totalOrders, pendingOrders, shippedOrders int64
	var totalSales float64

	u.db.WithContext(ctx).Model(&domainOrder.Order{}).Count(&totalOrders)
	u.db.WithContext(ctx).Model(&domainOrder.Order{}).Where("status = ?", domainOrder.StatusPending).Count(&pendingOrders)
	u.db.WithContext(ctx).Model(&domainOrder.Order{}).Where("status = ?", domainOrder.StatusShipped).Count(&shippedOrders)
	u.db.WithContext(ctx).Model(&domainOrder.Order{}).Where("status = ?", domainOrder.StatusShipped).Select("COALESCE(SUM(total_amount), 0)").Scan(&totalSales)

	summary.TotalOrders = totalOrders
	summary.PendingOrders = pendingOrders
	summary.ShippedOrders = shippedOrders
	summary.TotalSales = totalSales

	// SKUs, Suppliers, Customers
	u.db.WithContext(ctx).Model(&domainSKU.SKU{}).Count(&summary.TotalSKUs)
	u.db.WithContext(ctx).Model(&domainPurchasing.Supplier{}).Count(&summary.TotalSuppliers)
	u.db.WithContext(ctx).Model(&domainCustomer.Customer{}).Count(&summary.TotalCustomers)

	// Invoices
	u.db.WithContext(ctx).Model(&domainInvoice.Invoice{}).Where("status = ?", domainInvoice.StatusUnpaid).Count(&summary.TotalUnpaidInvs)
	u.db.WithContext(ctx).Model(&domainInvoice.Invoice{}).Where("status = ?", domainInvoice.StatusUnpaid).Select("COALESCE(SUM(amount), 0)").Scan(&summary.UnpaidInvsAmount)

	return &summary, nil
}

func (u *reportUsecase) GetRevenueReport(ctx context.Context, month string) (*domainReport.RevenueReport, error) {
	// Manual revenue is recognized only from fully paid ERP invoices. An
	// invoice without an order is treated as Manual; linked invoices are Manual
	// only when the source order is direct/manual. TikTok revenue comes directly
	// from the synced tiktok_orders table and is recognized only after TikTok
	// reports COMPLETED. Shopee is intentionally excluded for now.
	type manualRevenueRecord struct {
		InvoiceNo    string
		CustomerName string
		Amount       float64
		PaidAt       *time.Time
		CreatedAt    time.Time
	}

	var manualInvoices []manualRevenueRecord
	manualQuery := u.db.WithContext(ctx).
		Table("invoices AS invoices").
		Select(`invoices.invoice_no,
			invoices.customer_name,
			invoices.amount,
			invoices.paid_at,
			invoices.created_at`).
		Joins("LEFT JOIN orders AS orders ON orders.id = invoices.order_id").
		Where("invoices.status = ?", domainInvoice.StatusPaid).
		Where("invoices.order_id IS NULL OR LOWER(TRIM(COALESCE(orders.channel, ''))) IN ?", []string{"", "direct", "manual"})

	var tiktokOrders []domainTikTok.TiktokOrder
	tiktokQuery := u.db.WithContext(ctx).Model(&domainTikTok.TiktokOrder{}).
		Where("UPPER(TRIM(status)) IN ?", []string{"COMPLETED", "DELIVERED", "IN_TRANSIT", "AWAITING_SHIPMENT"})

	if month != "" {
		startTime := month + "-01"
		if parsed, err := time.Parse("2006-01-02", startTime); err == nil {
			endTime := parsed.AddDate(0, 1, 0).Format("2006-01-02")
			manualQuery = manualQuery.Where("COALESCE(invoices.paid_at, invoices.created_at) >= ? AND COALESCE(invoices.paid_at, invoices.created_at) < ?", startTime, endTime)
			tiktokQuery = tiktokQuery.Where("date >= ? AND date < ?", startTime, endTime)
		}
	}

	if err := manualQuery.Order("COALESCE(invoices.paid_at, invoices.created_at) DESC, invoices.id DESC").Scan(&manualInvoices).Error; err != nil {
		return nil, err
	}
	if err := tiktokQuery.Order("date DESC, id DESC").Find(&tiktokOrders).Error; err != nil {
		return nil, err
	}

	report := &domainReport.RevenueReport{
		Rows:      make([]domainReport.RevenueRow, 0, len(manualInvoices)+len(tiktokOrders)),
		Total:     0,
		ByChannel: map[string]float64{"Manual": 0, "TikTok": 0},
	}

	for _, inv := range manualInvoices {
		revDate := inv.CreatedAt
		if inv.PaidAt != nil {
			revDate = *inv.PaidAt
		}
		dateStr := revDate.Format("2006-01-02")
		report.Rows = append(report.Rows, domainReport.RevenueRow{
			Date:      dateStr,
			Reference: inv.InvoiceNo,
			Customer:  inv.CustomerName,
			Channel:   "Manual",
			Amount:    inv.Amount,
		})
		report.Total += inv.Amount
		report.ByChannel["Manual"] += inv.Amount
	}

	for _, o := range tiktokOrders {
		report.Rows = append(report.Rows, domainReport.RevenueRow{
			Date:      o.Date,
			Reference: o.ID,
			Customer:  "—",
			Channel:   "TikTok",
			Amount:    o.Amount,
		})
		report.Total += o.Amount
		report.ByChannel["TikTok"] += o.Amount
	}

	// Both sources are already sorted independently. Re-sort the combined rows
	// so the dashboard's recent-sales table remains newest-first.
	sort.SliceStable(report.Rows, func(i, j int) bool {
		if report.Rows[i].Date == report.Rows[j].Date {
			return report.Rows[i].Reference > report.Rows[j].Reference
		}
		return report.Rows[i].Date > report.Rows[j].Date
	})

	return report, nil
}

func (u *reportUsecase) GetFinancialSummary(ctx context.Context, month string) (*domainReport.FinancialSummary, error) {
	revReport, err := u.GetRevenueReport(ctx, month)
	if err != nil {
		return nil, err
	}

	revenue := revReport.Total

	// API-27: Calculate actual COGS by summing (order_items.quantity * skus.cost_price)
	var cogs float64
	cogsQuery := u.db.WithContext(ctx).Table("order_items").
		Select("COALESCE(SUM(order_items.quantity * skus.cost_price), 0)").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Joins("JOIN skus ON skus.sku = order_items.sku").
		Where("orders.status IN ?", []string{string(domainOrder.StatusShipped), "COMPLETED"})

	if month != "" {
		startTime := month + "-01"
		if parsed, err := time.Parse("2006-01-02", startTime); err == nil {
			endTime := parsed.AddDate(0, 1, 0).Format("2006-01-02")
			cogsQuery = cogsQuery.Where("orders.created_at >= ? AND orders.created_at < ?", startTime, endTime)
		}
	}
	_ = cogsQuery.Scan(&cogs).Error

	grossProfit := revenue - cogs
	operatingExpenses := 0.0
	damageLoss := 0.0
	netProfit := grossProfit - operatingExpenses - damageLoss

	grossMargin := 0.0
	netMargin := 0.0
	if revenue > 0 {
		grossMargin = (grossProfit / revenue) * 100
		netMargin = (netProfit / revenue) * 100
	}

	return &domainReport.FinancialSummary{
		Revenue:           revenue,
		COGS:              cogs,
		GrossProfit:       grossProfit,
		OperatingExpenses: operatingExpenses,
		DamageLoss:        damageLoss,
		NetProfit:         netProfit,
		GrossMargin:       grossMargin,
		NetMargin:         netMargin,
	}, nil
}

func (u *reportUsecase) GetInventoryValuation(ctx context.Context) (*domainReport.InventoryValuation, error) {
	type stockSKUJoin struct {
		SKUCode   string
		SKUName   string
		Quantity  int
		CostPrice float64
		Category  string
	}

	var results []stockSKUJoin
	err := u.db.WithContext(ctx).Table("stocks").
		Select("skus.sku as sku_code, skus.name as sku_name, stocks.quantity, skus.cost_price, skus.category").
		Joins("JOIN skus ON skus.id = stocks.sku_id").
		Where("stocks.quantity > 0").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	report := &domainReport.InventoryValuation{
		Rows:       make([]domainReport.InventoryValuationRow, 0, len(results)),
		TotalQty:   0,
		TotalValue: 0,
	}

	for _, r := range results {
		val := float64(r.Quantity) * r.CostPrice
		// API-28: Do not invent fake lot numbers or fake expiry dates when lot tracking is not yet active
		report.Rows = append(report.Rows, domainReport.InventoryValuationRow{
			SKU:          r.SKUCode,
			ProductName:  r.SKUName,
			Lot:          "—",
			ExpiryDate:   "—",
			RemainingQty: r.Quantity,
			UnitCost:     r.CostPrice,
			Value:        val,
		})
		report.TotalQty += r.Quantity
		report.TotalValue += val
	}

	return report, nil
}
