package report

import (
	"context"
	"sort"
	"time"

	domainReport "chawy-erp-api/internal/domain/report"
)

type Usecase interface {
	GetDashboardSummary(ctx context.Context) (*domainReport.DashboardSummary, error)
	GetRevenueReport(ctx context.Context, month string) (*domainReport.RevenueReport, error)
	GetFinancialSummary(ctx context.Context, month string) (*domainReport.FinancialSummary, error)
	GetInventoryValuation(ctx context.Context) (*domainReport.InventoryValuation, error)
}

type reportUsecase struct {
	repo domainReport.Repository
}

func NewReportUsecase(repo domainReport.Repository) Usecase {
	return &reportUsecase{repo: repo}
}

func (u *reportUsecase) GetDashboardSummary(ctx context.Context) (*domainReport.DashboardSummary, error) {
	return u.repo.GetDashboardSummary(ctx)
}

func (u *reportUsecase) GetRevenueReport(ctx context.Context, month string) (*domainReport.RevenueReport, error) {
	var startTime, endTime string
	if month != "" {
		st := month + "-01"
		if parsed, err := time.Parse("2006-01-02", st); err == nil {
			startTime = st
			endTime = parsed.AddDate(0, 1, 0).Format("2006-01-02")
		}
	}

	manualInvoices, err := u.repo.GetPaidManualInvoices(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

	tiktokOrders, err := u.repo.GetCompletedTikTokOrders(ctx, startTime, endTime)
	if err != nil {
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

	var startTime, endTime string
	if month != "" {
		st := month + "-01"
		if parsed, err := time.Parse("2006-01-02", st); err == nil {
			startTime = st
			endTime = parsed.AddDate(0, 1, 0).Format("2006-01-02")
		}
	}

	cogs, err := u.repo.GetCOGS(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}

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
	results, err := u.repo.GetStockValuationItems(ctx)
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
