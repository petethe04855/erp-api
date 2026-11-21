package postgres_test

import (
	"context"
	"testing"

	"chawy-erp-api/internal/domain/salesanalytics"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	"chawy-erp-api/internal/repository/postgres"

	"github.com/stretchr/testify/require"
)

func TestTikTokUpsertKeepsBuyerUserIDAndAnalyticsSendsIt(t *testing.T) {
	db := setupCRMTestDB(t)
	repo := postgres.NewTikTokRepository(db)
	ctx := context.Background()

	order := func(buyer string) []domainTikTok.TiktokOrder {
		return []domainTikTok.TiktokOrder{{
			ID: "TT-1", Date: "2026-11-10 10:00:00", Product: "ขนมไก่", SKU: "CHK-01", Qty: 1, Amount: 99,
			Status: "DELIVERED", RecipientProvince: "กรุงเทพมหานคร", BuyerUserID: buyer,
			Items: []domainTikTok.TiktokOrderItem{{OrderID: "TT-1", LineItemID: "L1", SKU: "CHK-01", ProductName: "ขนมไก่", Qty: 1, Amount: 99}},
		}}
	}
	require.NoError(t, repo.UpsertOrders(ctx, order("7494000000000001")))
	// A later sync or import without the id must not erase it.
	require.NoError(t, repo.UpsertOrders(ctx, order("")))

	var stored domainTikTok.TiktokOrder
	require.NoError(t, db.First(&stored, "id = ?", "TT-1").Error)
	require.Equal(t, "7494000000000001", stored.BuyerUserID)

	facts, err := postgres.NewSalesAnalyticsRepository(db).SalesOrders(ctx, salesanalytics.Query{
		DateFrom: "2026-11-01", DateTo: "2026-11-30", Channel: "tiktok",
	})
	require.NoError(t, err)
	require.Len(t, facts, 1)
	require.Equal(t, "7494000000000001", facts[0].BuyerID)
	require.Equal(t, "7494000000000001", facts[0].Buyer)
}
