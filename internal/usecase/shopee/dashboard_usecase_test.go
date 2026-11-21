package shopee_test

import (
	"context"
	"testing"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
	usecaseShopee "chawy-erp-api/internal/usecase/shopee"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubMatching returns fixed rows per month for the dashboard aggregation.
type stubMatching struct {
	rows map[int][]domainShopee.MatchingItemRow
}

func (s *stubMatching) GetMonthlyMatching(_ context.Context, _ int, month int) ([]domainShopee.MatchingItemRow, *domainShopee.MonthlySummary, error) {
	var gross float64
	for _, r := range s.rows[month] {
		gross += r.LineSale
	}
	return s.rows[month], &domainShopee.MonthlySummary{GrossSale: gross}, nil
}

func (s *stubMatching) ExportMonthlyMatchingCSV(context.Context, int, int) ([]byte, string, error) {
	return nil, "", nil
}

func day(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func TestDashboardUsecase_DailySalesByTransferDate(t *testing.T) {
	m := &stubMatching{rows: map[int][]domainShopee.MatchingItemRow{
		3: {
			{SKU: "A", TransferDate: day("2026-03-05"), LineSale: 100, AllocatedNet: 90, Profit: 30},
			{SKU: "B", TransferDate: day("2026-03-05"), LineSale: 50, AllocatedNet: 45, Profit: 25},
			{SKU: "A", TransferDate: day("2026-03-02"), LineSale: 80, AllocatedNet: 70, Profit: 20},
		},
		4: {
			// payout without an order: no gross, but net and profit count
			{TransferDate: day("2026-04-01"), AllocatedNet: 40, Profit: 40, Status: "MISSING_ORDER"},
		},
	}}

	res, err := usecaseShopee.NewDashboardUsecase(m).GetDashboardAnalytics(context.Background(), 2026, 3, 2026, 4)
	require.NoError(t, err)

	require.Len(t, res.DailySales, 3)
	assert.Equal(t, usecaseShopee.DailySale{Date: "2026-03-02", GrossSale: 80, NetReceive: 70, NetProfit: 20}, res.DailySales[0])
	assert.Equal(t, usecaseShopee.DailySale{Date: "2026-03-05", GrossSale: 150, NetReceive: 135, NetProfit: 55}, res.DailySales[1])
	assert.Equal(t, usecaseShopee.DailySale{Date: "2026-04-01", GrossSale: 0, NetReceive: 40, NetProfit: 40}, res.DailySales[2])

	// The daily series adds up to the summary gross.
	var sum float64
	for _, d := range res.DailySales {
		sum += d.GrossSale
	}
	assert.Equal(t, res.YTDSummary.GrossSale, sum)
}
