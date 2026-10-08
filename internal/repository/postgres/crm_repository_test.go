package postgres_test

import (
	"context"
	"testing"
	"time"

	"chawy-erp-api/internal/domain/crm"
	domainShopee "chawy-erp-api/internal/domain/shopee"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	"chawy-erp-api/internal/repository/postgres"
	"chawy-erp-api/pkg/province"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCRMTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&domainTikTok.TiktokOrder{},
		&domainTikTok.TiktokOrderItem{},
		&domainShopee.ShopeeOrder{},
		&domainShopee.ShopeeOrderItem{},
	)
	require.NoError(t, err)

	return db
}

func TestGetTiktokProvinceReport(t *testing.T) {
	db := setupCRMTestDB(t)
	repo := postgres.NewCRMRepository(db)
	ctx := context.Background()

	orders := []domainTikTok.TiktokOrder{
		{
			ID:                "TK001",
			Date:              "2026-10-01",
			Status:            "DELIVERED",
			RecipientProvince: "กรุงเทพมหานคร",
			Qty:               2,
			Amount:            500.0,
		},
		{
			ID:                "TK002",
			Date:              "2026-10-02",
			Status:            "COMPLETED",
			RecipientProvince: "กรุงเทพมหานคร",
			Qty:               1,
			Amount:            250.0,
		},
		{
			ID:                "TK003",
			Date:              "2026-10-03",
			Status:            "IN_TRANSIT",
			RecipientProvince: "ชลบุรี",
			Qty:               3,
			Amount:            1000.0,
		},
		{
			ID:                "TK004",
			Date:              "2026-10-04",
			Status:            "CANCELLED",
			RecipientProvince: "ภูเก็ต",
			Qty:               1,
			Amount:            300.0,
		},
		{
			ID:                "TK005",
			Date:              "2026-10-05",
			Status:            "DELIVERED",
			RecipientProvince: "", // empty -> should map to ไม่ทราบจังหวัด
			Qty:               1,
			Amount:            200.0,
		},
	}

	for _, o := range orders {
		require.NoError(t, db.Create(&o).Error)
	}

	t.Run("Default fulfilled status", func(t *testing.T) {
		report, err := repo.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{
			DateFrom: "2026-10-01",
			DateTo:   "2026-10-05",
			Status:   "fulfilled",
		})
		require.NoError(t, err)
		require.NotNil(t, report)

		// TK001, TK002, TK003, TK005 (TK004 CANCELLED is excluded)
		assert.Equal(t, int64(4), report.Summary.TotalOrders)
		assert.Equal(t, int64(7), report.Summary.TotalItemQty)
		assert.Equal(t, 1950.0, report.Summary.GrossSales)
		assert.Equal(t, "กรุงเทพมหานคร", report.Summary.TopProvince)
		assert.Equal(t, 2, report.Summary.ProvinceCount) // BKK, Chonburi
		assert.Equal(t, 75.0, report.Summary.DataCompletenessPercent)

		require.Len(t, report.Provinces, 3)

		// First row: BKK (2 orders)
		assert.Equal(t, "กรุงเทพมหานคร", report.Provinces[0].Province)
		assert.Equal(t, int64(2), report.Provinces[0].OrderCount)
		assert.Equal(t, int64(3), report.Provinces[0].ItemQty)
		assert.Equal(t, 750.0, report.Provinces[0].GrossSales)
		assert.Equal(t, 50.0, report.Provinces[0].SharePercent)

		// Second row: Chonburi (1 order, 1000฿) or Unknown (1 order, 200฿)
		assert.Equal(t, "ชลบุรี", report.Provinces[1].Province)
		assert.Equal(t, int64(1), report.Provinces[1].OrderCount)
		assert.Equal(t, 1000.0, report.Provinces[1].GrossSales)
		assert.Equal(t, 25.0, report.Provinces[1].SharePercent)

		// Third row: Unknown province
		assert.Equal(t, province.UnknownProvince, report.Provinces[2].Province)
		assert.Equal(t, int64(1), report.Provinces[2].OrderCount)
		assert.Equal(t, 200.0, report.Provinces[2].GrossSales)
		assert.Equal(t, 25.0, report.Provinces[2].SharePercent)
	})

	t.Run("Cancelled filter", func(t *testing.T) {
		report, err := repo.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{
			DateFrom: "2026-10-01",
			DateTo:   "2026-10-05",
			Status:   "cancelled",
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), report.Summary.TotalOrders)
		assert.Equal(t, "ภูเก็ต", report.Summary.TopProvince)
		require.Len(t, report.Provinces, 1)
		assert.Equal(t, "ภูเก็ต", report.Provinces[0].Province)
		assert.Equal(t, 100.0, report.Provinces[0].SharePercent)
	})

	t.Run("Specific province filter", func(t *testing.T) {
		report, err := repo.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{
			DateFrom: "2026-10-01",
			DateTo:   "2026-10-05",
			Status:   "fulfilled",
			Province: "ชลบุรี",
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), report.Summary.TotalOrders)
		assert.Equal(t, "ชลบุรี", report.Summary.TopProvince)
		require.Len(t, report.Provinces, 1)
		assert.Equal(t, "ชลบุรี", report.Provinces[0].Province)
	})
}

func seedShopeeOrders(t *testing.T, db *gorm.DB) {
	t.Helper()
	mk := func(id, prov string, day int, status string, lines ...domainShopee.ShopeeOrderItem) {
		ord := domainShopee.ShopeeOrder{
			ID:            id,
			OrderDate:     time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC),
			BuyerUsername: "buyer",
			Province:      prov,
			Status:        status,
			RowHash:       "hash-" + id,
			Items:         lines,
		}
		require.NoError(t, db.Create(&ord).Error)
	}

	mk("SP001", "ชลบุรี", 2, "COMPLETED",
		domainShopee.ShopeeOrderItem{OrderID: "SP001", SKU: "A", ProductName: "A", Qty: 2, SalePrice: 400})
	mk("SP002", "กรุงเทพมหานคร", 3, "COMPLETED",
		domainShopee.ShopeeOrderItem{OrderID: "SP002", SKU: "B", ProductName: "B", Qty: 1, SalePrice: 150})
	mk("SP003", "", 4, "COMPLETED", // unknown province
		domainShopee.ShopeeOrderItem{OrderID: "SP003", SKU: "C", ProductName: "C", Qty: 1, SalePrice: 90})
}

func TestGetProvinceReportChannelFilter(t *testing.T) {
	db := setupCRMTestDB(t)
	repo := postgres.NewCRMRepository(db)
	ctx := context.Background()

	// TikTok: BKK x1, Chonburi x1 (fulfilled)
	for _, o := range []domainTikTok.TiktokOrder{
		{ID: "TK101", Date: "2026-10-01", Status: "DELIVERED", RecipientProvince: "กรุงเทพมหานคร", Qty: 1, Amount: 500},
		{ID: "TK102", Date: "2026-10-02", Status: "COMPLETED", RecipientProvince: "ชลบุรี", Qty: 2, Amount: 300},
	} {
		require.NoError(t, db.Create(&o).Error)
	}
	seedShopeeOrders(t, db)

	run := func(channel string) *crm.TiktokProvinceReport {
		t.Helper()
		rep, err := repo.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{
			DateFrom: "2026-10-01",
			DateTo:   "2026-10-05",
			Status:   "fulfilled",
			Channel:  channel,
		})
		require.NoError(t, err)
		return rep
	}

	t.Run("tiktok only", func(t *testing.T) {
		rep := run("tiktok")
		assert.Equal(t, int64(2), rep.Summary.TotalOrders)
		assert.Equal(t, int64(3), rep.Summary.TotalItemQty)
		assert.Equal(t, 800.0, rep.Summary.GrossSales)
		require.Len(t, rep.Provinces, 2)
	})

	t.Run("shopee only", func(t *testing.T) {
		rep := run("shopee")
		assert.Equal(t, int64(3), rep.Summary.TotalOrders)
		assert.Equal(t, int64(4), rep.Summary.TotalItemQty)
		assert.Equal(t, 640.0, rep.Summary.GrossSales)
		require.Len(t, rep.Provinces, 3) // Chonburi, BKK, Unknown
	})

	t.Run("all merges channels by province", func(t *testing.T) {
		rep := run("") // default = all
		// 5 orders total (2 TikTok + 3 Shopee)
		assert.Equal(t, int64(5), rep.Summary.TotalOrders)
		assert.Equal(t, int64(7), rep.Summary.TotalItemQty)
		assert.Equal(t, 1440.0, rep.Summary.GrossSales)

		// Chonburi: TK102 (1 order, 300฿) + SP001 (1 order, 400฿) = 2 orders, 700฿
		// ties BKK on order count (2) but wins the gross-sales tie-break (650฿)
		require.Len(t, rep.Provinces, 3) // Chonburi, BKK, Unknown (merged)
		assert.Equal(t, "ชลบุรี", rep.Provinces[0].Province)
		assert.Equal(t, int64(2), rep.Provinces[0].OrderCount)
		assert.Equal(t, 700.0, rep.Provinces[0].GrossSales)
		assert.Equal(t, "ชลบุรี", rep.Summary.TopProvince)
	})
}
