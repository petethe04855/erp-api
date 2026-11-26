package postgres_test

import (
	"context"
	"testing"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
	"chawy-erp-api/internal/repository/postgres"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupShopeeOrderDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domainShopee.ShopeeOrder{}, &domainShopee.ShopeeOrderItem{}, &domainShopee.ShopeeOrderRepair{}))
	return db
}

func seedOldParserOrder(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&domainShopee.ShopeeOrder{
		ID: "A", OrderDate: time.Date(2026, 7, 2, 10, 15, 0, 0, time.UTC), Status: "COMPLETED", RowHash: "h",
	}).Error)
	require.NoError(t, db.Create(&[]domainShopee.ShopeeOrderItem{
		{OrderID: "A", SKU: "FD-03", OriginalSKU: "FD-03", ProductName: "ฟรีซดราย", Qty: 1, SalePrice: 918},
		// The user corrected this SKU and confirmed it.
		{OrderID: "A", SKU: "TOY-FIXED", OriginalSKU: "TOY-1", ProductName: "ไม้ตกแมว", Qty: 1, SalePrice: 59, SKUConfirmed: true},
	}).Error)
}

// Re-importing a file with the corrected quantity (or the original SKU of a
// line whose SKU was edited) must not add the line a second time.
func TestShopeeBulkInsert_ReimportDoesNotDuplicateLines(t *testing.T) {
	db := setupShopeeOrderDB(t)
	seedOldParserOrder(t, db)
	repo := postgres.NewShopeeOrderRepository(db)

	inserted, skipped, err := repo.BulkInsert(context.Background(), []domainShopee.ShopeeOrder{{
		ID: "A", OrderDate: time.Date(2026, 6, 30, 14, 9, 0, 0, time.UTC), Status: "COMPLETED",
		Items: []domainShopee.ShopeeOrderItem{
			{SKU: "FD-03", OriginalSKU: "FD-03", ProductName: "ฟรีซดราย", Qty: 2, SalePrice: 918},
			{SKU: "TOY-1", OriginalSKU: "TOY-1", ProductName: "ไม้ตกแมว", Qty: 1, SalePrice: 59},
		},
	}})
	require.NoError(t, err)
	assert.Equal(t, 0, inserted)
	assert.Equal(t, 1, skipped)

	var count int64
	db.Model(&domainShopee.ShopeeOrderItem{}).Where("order_id = ?", "A").Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestShopeeApplyRepairs_UpdatesInPlaceAndLogs(t *testing.T) {
	db := setupShopeeOrderDB(t)
	seedOldParserOrder(t, db)
	repo := postgres.NewShopeeOrderRepository(db)
	ctx := context.Background()

	stored, err := repo.FindByOrderIDs(ctx, []string{"A"})
	require.NoError(t, err)
	require.Len(t, stored, 1)
	fd := stored[0].Items[0]
	newDate := time.Date(2026, 6, 30, 21, 9, 0, 0, time.FixedZone("ICT", 7*3600))

	err = repo.ApplyRepairs(ctx, []domainShopee.OrderRepair{{
		OrderID: "A", OldOrderDate: stored[0].OrderDate, NewOrderDate: newDate, DateChanged: true,
		Items: []domainShopee.OrderItemRepair{{ItemID: fd.ID, OldQty: 1, NewQty: 2, OldVariation: "", NewVariation: "3 ถุง"}},
	}}, "repair-test", "owner@chawy.test", "Order.completed.xlsx")
	require.NoError(t, err)

	after, err := repo.FindByOrderIDs(ctx, []string{"A"})
	require.NoError(t, err)
	assert.True(t, after[0].OrderDate.Equal(newDate))
	var freezeDried, toy domainShopee.ShopeeOrderItem
	for _, it := range after[0].Items {
		if it.ID == fd.ID {
			freezeDried = it
		} else {
			toy = it
		}
	}
	assert.Equal(t, 2, freezeDried.Qty)
	assert.Equal(t, "3 ถุง", freezeDried.VariationName)
	assert.Equal(t, 1, toy.Qty)
	assert.Equal(t, "TOY-FIXED", toy.SKU, "a corrected SKU is never touched")
	assert.True(t, toy.SKUConfirmed)

	var logs []domainShopee.ShopeeOrderRepair
	require.NoError(t, db.Order("id").Find(&logs).Error)
	if assert.Len(t, logs, 3) {
		assert.Equal(t, "order_date", logs[0].Field)
		assert.Equal(t, "qty", logs[1].Field)
		assert.Equal(t, "1", logs[1].OldValue)
		assert.Equal(t, "2", logs[1].NewValue)
		assert.Equal(t, "variation_name", logs[2].Field)
		assert.Equal(t, "owner@chawy.test", logs[2].Actor)
		assert.Equal(t, "repair-test", logs[2].BatchID)
	}
}

// When a row no longer holds the value seen in the preview, nothing is saved.
func TestShopeeApplyRepairs_StalePreviewRollsBackEverything(t *testing.T) {
	db := setupShopeeOrderDB(t)
	seedOldParserOrder(t, db)
	repo := postgres.NewShopeeOrderRepository(db)
	ctx := context.Background()

	stored, err := repo.FindByOrderIDs(ctx, []string{"A"})
	require.NoError(t, err)
	fd := stored[0].Items[0]

	err = repo.ApplyRepairs(ctx, []domainShopee.OrderRepair{{
		OrderID: "A", OldOrderDate: stored[0].OrderDate, NewOrderDate: stored[0].OrderDate.Add(-time.Hour), DateChanged: true,
		// Says the stored qty is 5, but it is 1.
		Items: []domainShopee.OrderItemRepair{{ItemID: fd.ID, OldQty: 5, NewQty: 2}},
	}}, "repair-test", "owner@chawy.test", "f.xlsx")
	assert.Error(t, err)

	after, _ := repo.FindByOrderIDs(ctx, []string{"A"})
	assert.True(t, after[0].OrderDate.Equal(stored[0].OrderDate), "the date change is rolled back too")
	var logs int64
	db.Model(&domainShopee.ShopeeOrderRepair{}).Count(&logs)
	assert.Equal(t, int64(0), logs)
}

func TestShopeeListInsightLines_CompletedBeforeUntil(t *testing.T) {
	db := setupShopeeOrderDB(t)
	day := func(d int) time.Time { return time.Date(2026, 6, d, 12, 0, 0, 0, time.UTC) }
	require.NoError(t, db.Create(&[]domainShopee.ShopeeOrder{
		{ID: "A", OrderDate: day(2), Status: "COMPLETED", BuyerUsername: "u1", RowHash: "a"},
		{ID: "B", OrderDate: day(1), Status: "COMPLETED", BuyerUsername: "u2", RowHash: "b"},
		{ID: "C", OrderDate: day(3), Status: "CANCELLED", RowHash: "c"},
		{ID: "D", OrderDate: day(20), Status: "COMPLETED", RowHash: "d"},
	}).Error)
	require.NoError(t, db.Create(&[]domainShopee.ShopeeOrderItem{
		{OrderID: "A", SKU: "S1", ProductName: "อกไก่ฟรีซดราย", VariationName: "3 ถุง", Qty: 2, SalePrice: 600},
		{OrderID: "B", SKU: "S2", ProductName: "ไม้ตกแมว", Qty: 1, SalePrice: 59},
		{OrderID: "C", SKU: "S3", ProductName: "x", Qty: 1, SalePrice: 1},
		{OrderID: "D", SKU: "S4", ProductName: "y", Qty: 1, SalePrice: 1},
	}).Error)

	lines, err := postgres.NewShopeeInsightRepository(db).ListInsightLines(context.Background(), day(10))
	require.NoError(t, err)
	require.Len(t, lines, 2, "cancelled orders and orders on or after `until` are left out")
	assert.Equal(t, "B", lines[0].OrderID, "oldest first")
	assert.Equal(t, "u1", lines[1].BuyerUsername)
	assert.Equal(t, "3 ถุง", lines[1].VariationName)
	assert.Equal(t, 2, lines[1].Qty)
	assert.Equal(t, 600.0, lines[1].SalePrice)
}
