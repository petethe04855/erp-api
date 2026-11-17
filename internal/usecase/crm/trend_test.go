package crm

import (
	"testing"
	"time"

	"chawy-erp-api/internal/domain/crm"

	"github.com/stretchr/testify/assert"
)

func TestBuildSalesTrend(t *testing.T) {
	loc := time.FixedZone("Asia/Bangkok", 7*60*60)
	from := time.Date(2026, 3, 25, 0, 0, 0, 0, loc) // Wednesday, ISO week 13
	to := time.Date(2026, 4, 8, 0, 0, 0, 0, loc)    // Wednesday, ISO week 15

	trend := buildSalesTrend([]crm.DailyChannelSales{
		{Day: "2026-03-30", Channel: "tiktok", OrderCount: 3, GrossSales: 300},
		{Day: "2026-04-01", Channel: "shopee", OrderCount: 2, GrossSales: 150.555},
		{Day: "2026-04-05", Channel: "tiktok", OrderCount: 1, GrossSales: 100},
		{Day: "bad", Channel: "tiktok", OrderCount: 9, GrossSales: 999},
	}, from, to)

	assert.Len(t, trend.Weekly, 3)
	assert.Equal(t, "2026-W13", trend.Weekly[0].Period)
	assert.Equal(t, "2026-03-23", trend.Weekly[0].StartDate)
	assert.Equal(t, int64(0), trend.Weekly[0].TotalOrders)

	w14 := trend.Weekly[1]
	assert.Equal(t, "2026-W14", w14.Period)
	assert.Equal(t, "2026-03-30", w14.StartDate)
	assert.Equal(t, "2026-04-05", w14.EndDate)
	assert.Equal(t, 400.0, w14.TiktokSales)
	assert.Equal(t, 150.56, w14.ShopeeSales)
	assert.Equal(t, 550.56, w14.TotalSales)
	assert.Equal(t, int64(6), w14.TotalOrders)

	assert.Len(t, trend.Monthly, 2)
	assert.Equal(t, "2026-03", trend.Monthly[0].Period)
	assert.Equal(t, "2026-03-31", trend.Monthly[0].EndDate)
	assert.Equal(t, 300.0, trend.Monthly[0].TiktokSales)
	assert.Equal(t, int64(3), trend.Monthly[1].TotalOrders)
}
