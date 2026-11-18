package shopee

import (
	"context"
	"fmt"
	"sort"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
)

type DashboardAnalyticsResult struct {
	FromMonth     string                        `json:"from_month"`
	ToMonth       string                        `json:"to_month"`
	YTDSummary    domainShopee.MonthlySummary   `json:"ytd_summary"`
	MonthlyTrends []domainShopee.MonthlySummary `json:"monthly_trends"`
	TopSKUs       []domainShopee.TopSKUStat     `json:"top_skus"`
	DailySales    []DailySale                   `json:"daily_sales"`
}

// DailySale is Shopee sales per transfer date. Gross sums the order lines of
// the payouts received that day, so the series adds up to the summary GrossSale.
type DailySale struct {
	Date       string  `json:"date"` // YYYY-MM-DD (transfer date)
	GrossSale  float64 `json:"gross_sale"`
	NetReceive float64 `json:"net_receive"`
	NetProfit  float64 `json:"net_profit"`
}

type DashboardUsecase interface {
	GetDashboardAnalytics(ctx context.Context, fromYear, fromMonth, toYear, toMonth int) (*DashboardAnalyticsResult, error)
}

type dashboardUsecase struct {
	matchingUsecase MatchingUsecase
}

func NewDashboardUsecase(matchingUsecase MatchingUsecase) DashboardUsecase {
	return &dashboardUsecase{
		matchingUsecase: matchingUsecase,
	}
}

func (u *dashboardUsecase) GetDashboardAnalytics(ctx context.Context, fromYear, fromMonth, toYear, toMonth int) (*DashboardAnalyticsResult, error) {
	startDate := time.Date(fromYear, time.Month(fromMonth), 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(toYear, time.Month(toMonth), 1, 0, 0, 0, 0, time.UTC)

	if endDate.Before(startDate) {
		startDate, endDate = endDate, startDate
	}

	var monthlyTrends []domainShopee.MonthlySummary
	skuMap := make(map[string]*domainShopee.TopSKUStat)
	dailyMap := make(map[string]*DailySale)

	curr := startDate
	var ytdGross, ytdFees, ytdNet, ytdCost, ytdProfit, ytdMatchedProfit float64
	var ytdOrders, ytdItems int

	for !curr.After(endDate) {
		y := curr.Year()
		m := int(curr.Month())

		rows, summary, err := u.matchingUsecase.GetMonthlyMatching(ctx, y, m)
		if err != nil {
			return nil, err
		}

		if summary != nil {
			monthlyTrends = append(monthlyTrends, *summary)
			ytdGross += summary.GrossSale
			ytdFees += summary.PlatformFees
			ytdNet += summary.NetReceive
			ytdCost += summary.TotalCost
			ytdProfit += summary.NetProfit
			ytdMatchedProfit += summary.MatchedNetProfit
			ytdOrders += summary.OrderCount
			ytdItems += summary.ItemCount
		}

		// Aggregate SKU statistics and daily sales
		for _, r := range rows {
			day := r.TransferDate.UTC().Format("2006-01-02") // same UTC bounds as FindByMonth
			d, ok := dailyMap[day]
			if !ok {
				d = &DailySale{Date: day}
				dailyMap[day] = d
			}
			d.GrossSale += r.LineSale
			d.NetReceive += r.AllocatedNet
			d.NetProfit += r.Profit

			if r.SKU == "" {
				continue
			}
			stat, exists := skuMap[r.SKU]
			if !exists {
				stat = &domainShopee.TopSKUStat{
					SKU:         r.SKU,
					ProductName: r.ProductName,
				}
				skuMap[r.SKU] = stat
			}
			stat.TotalQty += r.Qty
			stat.TotalGross += r.LineSale
			stat.TotalCost += r.TotalCost
			stat.NetProfit += r.Profit
		}

		curr = curr.AddDate(0, 1, 0)
	}

	ytdMargin := 0.0
	// Net Profit Margin = Net Profit / Gross Sales × 100
	if ytdGross > 0 {
		ytdMargin = round2((ytdMatchedProfit / ytdGross) * 100)
	}

	ytdSummary := domainShopee.MonthlySummary{
		Month:            fmt.Sprintf("%04d-%02d to %04d-%02d", startDate.Year(), startDate.Month(), endDate.Year(), endDate.Month()),
		OrderCount:       ytdOrders,
		ItemCount:        ytdItems,
		GrossSale:        round2(ytdGross),
		PlatformFees:     round2(ytdFees),
		NetReceive:       round2(ytdNet),
		TotalCost:        round2(ytdCost),
		NetProfit:        round2(ytdProfit),
		ProfitMarginPct:  ytdMargin,
		MatchedNetProfit: round2(ytdMatchedProfit),
	}

	// Sort Top SKUs by NetProfit DESC
	var topSKUList []domainShopee.TopSKUStat
	for _, stat := range skuMap {
		if stat.TotalGross > 0 {
			stat.MarginPct = round2((stat.NetProfit / stat.TotalGross) * 100)
		}
		stat.TotalGross = round2(stat.TotalGross)
		stat.TotalCost = round2(stat.TotalCost)
		stat.NetProfit = round2(stat.NetProfit)
		topSKUList = append(topSKUList, *stat)
	}

	sort.Slice(topSKUList, func(i, j int) bool {
		return topSKUList[i].NetProfit > topSKUList[j].NetProfit
	})

	if len(topSKUList) > 5 {
		topSKUList = topSKUList[:5]
	}

	dailySales := make([]DailySale, 0, len(dailyMap))
	for _, d := range dailyMap {
		dailySales = append(dailySales, DailySale{
			Date:       d.Date,
			GrossSale:  round2(d.GrossSale),
			NetReceive: round2(d.NetReceive),
			NetProfit:  round2(d.NetProfit),
		})
	}
	sort.Slice(dailySales, func(i, j int) bool { return dailySales[i].Date < dailySales[j].Date })

	return &DashboardAnalyticsResult{
		FromMonth:     fmt.Sprintf("%04d-%02d", startDate.Year(), startDate.Month()),
		ToMonth:       fmt.Sprintf("%04d-%02d", endDate.Year(), endDate.Month()),
		YTDSummary:    ytdSummary,
		MonthlyTrends: monthlyTrends,
		TopSKUs:       topSKUList,
		DailySales:    dailySales,
	}, nil
}
