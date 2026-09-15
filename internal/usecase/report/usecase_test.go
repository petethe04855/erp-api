package report_test

import (
	"context"
	"testing"
	"time"

	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	usecaseReport "chawy-erp-api/internal/usecase/report"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&domainOrder.Order{},
		&domainInvoice.Invoice{},
		&domainTikTok.TiktokOrder{},
	)
	require.NoError(t, err)

	return db
}

func TestGetRevenueReport_TikTokCompletedAndManualPaid(t *testing.T) {
	db := setupTestDB(t)
	uc := usecaseReport.NewReportUsecase(db)
	ctx := context.Background()

	// 1. Setup Orders
	orderDirect := domainOrder.Order{
		OrderNo:     "ORD-DIR-001",
		CustomerName: "Direct Customer",
		Channel:     "direct",
		Status:      domainOrder.StatusShipped,
	}
	orderManual := domainOrder.Order{
		OrderNo:     "ORD-MAN-001",
		CustomerName: "Manual Customer",
		Channel:     "manual",
		Status:      domainOrder.StatusShipped,
	}
	orderTikTok := domainOrder.Order{
		OrderNo:     "ORD-TT-001",
		CustomerName: "TikTok Customer",
		Channel:     "tiktok",
		Status:      domainOrder.StatusShipped,
	}
	orderShopee := domainOrder.Order{
		OrderNo:     "ORD-SP-001",
		CustomerName: "Shopee Customer",
		Channel:     "shopee",
		Status:      domainOrder.StatusShipped,
	}
	require.NoError(t, db.Create(&orderDirect).Error)
	require.NoError(t, db.Create(&orderManual).Error)
	require.NoError(t, db.Create(&orderTikTok).Error)
	require.NoError(t, db.Create(&orderShopee).Error)

	paidDateSep15 := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	paidDateAug20 := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)

	// 2. Setup Invoices
	invoices := []domainInvoice.Invoice{
		// Manual: Paid invoice with no order (should be included)
		{
			InvoiceNo:    "INV-NO-ORDER",
			CustomerName: "Walk-in",
			Amount:       500,
			Status:       domainInvoice.StatusPaid,
			PaidAt:       &paidDateSep15,
		},
		// Manual: Paid invoice with direct order (should be included)
		{
			InvoiceNo:    "INV-DIRECT",
			OrderID:      &orderDirect.ID,
			CustomerName: "Direct Customer",
			Amount:       1000,
			Status:       domainInvoice.StatusPaid,
			PaidAt:       &paidDateSep15,
		},
		// Manual: Paid invoice with manual order (should be included)
		{
			InvoiceNo:    "INV-MANUAL",
			OrderID:      &orderManual.ID,
			CustomerName: "Manual Customer",
			Amount:       1500,
			Status:       domainInvoice.StatusPaid,
			PaidAt:       &paidDateSep15,
		},
		// Manual: Legacy paid invoice without paid_at (fallback to created_at in Sep 2026)
		{
			InvoiceNo:    "INV-LEGACY",
			CustomerName: "Legacy Customer",
			Amount:       300,
			Status:       domainInvoice.StatusPaid,
			PaidAt:       nil,
			CreatedAt:    paidDateSep15,
		},
		// Unpaid invoice (should NOT be included)
		{
			InvoiceNo:    "INV-UNPAID",
			CustomerName: "Unpaid Customer",
			Amount:       999,
			Status:       domainInvoice.StatusUnpaid,
		},
		// Partially paid invoice (should NOT be included)
		{
			InvoiceNo:    "INV-PARTIAL",
			CustomerName: "Partial Customer",
			Amount:       888,
			Status:       domainInvoice.StatusPartiallyPaid,
		},
		// Cancelled invoice (should NOT be included)
		{
			InvoiceNo:    "INV-CANCELLED",
			CustomerName: "Cancelled Customer",
			Amount:       777,
			Status:       domainInvoice.StatusCancelled,
		},
		// Paid invoice linked to TikTok order (should NOT be included as manual)
		{
			InvoiceNo:    "INV-TT",
			OrderID:      &orderTikTok.ID,
			CustomerName: "TikTok Local Order",
			Amount:       600,
			Status:       domainInvoice.StatusPaid,
			PaidAt:       &paidDateSep15,
		},
		// Paid invoice linked to Shopee order (should NOT be included)
		{
			InvoiceNo:    "INV-SP",
			OrderID:      &orderShopee.ID,
			CustomerName: "Shopee Local Order",
			Amount:       400,
			Status:       domainInvoice.StatusPaid,
			PaidAt:       &paidDateSep15,
		},
		// August Paid invoice (should be filtered out when querying 2026-09)
		{
			InvoiceNo:    "INV-AUG",
			CustomerName: "August Customer",
			Amount:       200,
			Status:       domainInvoice.StatusPaid,
			PaidAt:       &paidDateAug20,
		},
	}
	for _, inv := range invoices {
		require.NoError(t, db.Create(&inv).Error)
	}

	// 3. Setup TikTok Orders
	ttOrders := []domainTikTok.TiktokOrder{
		// Completed (should be included)
		{
			ID:     "TT-COMPLETED-1",
			Date:   "2026-09-15",
			Status: "COMPLETED",
			Amount: 700,
		},
		// Completed with lowercase/spaces (should be included)
		{
			ID:     "TT-COMPLETED-2",
			Date:   "2026-09-16",
			Status: " completed ",
			Amount: 800,
		},
		// Delivered (should NOT be included)
		{
			ID:     "TT-DELIVERED",
			Date:   "2026-09-15",
			Status: "DELIVERED",
			Amount: 555,
		},
		// Shipped (should NOT be included)
		{
			ID:     "TT-SHIPPED",
			Date:   "2026-09-15",
			Status: "SHIPPED",
			Amount: 444,
		},
		// In Transit (should NOT be included)
		{
			ID:     "TT-TRANSIT",
			Date:   "2026-09-15",
			Status: "IN_TRANSIT",
			Amount: 333,
		},
		// Cancelled (should NOT be included)
		{
			ID:     "TT-CANCELLED",
			Date:   "2026-09-15",
			Status: "CANCELLED",
			Amount: 222,
		},
		// August Completed (should be filtered out when querying 2026-09)
		{
			ID:     "TT-AUG",
			Date:   "2026-08-15",
			Status: "COMPLETED",
			Amount: 111,
		},
	}
	for _, tt := range ttOrders {
		require.NoError(t, db.Create(&tt).Error)
	}

	// Execute Test for 2026-09
	report, err := uc.GetRevenueReport(ctx, "2026-09")
	require.NoError(t, err)
	require.NotNil(t, report)

	// Expected Manual:
	// INV-NO-ORDER: 500
	// INV-DIRECT: 1000
	// INV-MANUAL: 1500
	// INV-LEGACY: 300
	// Total Manual = 3300
	assert.Equal(t, 3300.0, report.ByChannel["Manual"])

	// Expected TikTok:
	// TT-COMPLETED-1: 700
	// TT-COMPLETED-2: 800
	// Total TikTok = 1500
	assert.Equal(t, 1500.0, report.ByChannel["TikTok"])

	// Shopee must not exist
	_, hasShopee := report.ByChannel["Shopee"]
	assert.False(t, hasShopee, "Shopee should not be in ByChannel")

	// Total
	assert.Equal(t, 4800.0, report.Total)

	// Rows count: 4 manual + 2 tiktok = 6
	assert.Len(t, report.Rows, 6)

	// Verify order sorting: newest date first, then reference descending
	for i := 0; i < len(report.Rows)-1; i++ {
		cur := report.Rows[i]
		next := report.Rows[i+1]
		if cur.Date == next.Date {
			assert.GreaterOrEqual(t, cur.Reference, next.Reference)
		} else {
			assert.Greater(t, cur.Date, next.Date)
		}
	}
}
