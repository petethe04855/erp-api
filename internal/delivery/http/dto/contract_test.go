package dto

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	domainAuth "chawy-erp-api/internal/domain/auth"
	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainFinance "chawy-erp-api/internal/domain/finance"
	domainLive "chawy-erp-api/internal/domain/live"
	domainReturn "chawy-erp-api/internal/domain/salesreturn"
	domainShopee "chawy-erp-api/internal/domain/shopee"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
)

// These tests freeze the wire contract introduced by the DTO boundary work.
// A response DTO must serialize to exactly the same JSON object as the domain
// entity it replaced, otherwise existing frontend call sites break silently.

func toMap(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func assertSameJSON(t *testing.T, name string, want, got interface{}) {
	t.Helper()
	w, g := toMap(t, want), toMap(t, got)
	if !reflect.DeepEqual(w, g) {
		t.Fatalf("%s: wire contract drift\n want: %#v\n  got: %#v", name, w, g)
	}
}

func TestBundleItemResponseMatchesDomainJSON(t *testing.T) {
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	item := domainBundle.BundleItem{
		ID:           7,
		BundleSKU:    "FD-MX-C1T2-03",
		ComponentSKU: "FD-TN-80-01",
		Quantity:     3,
		Note:         "ลัง",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assertSameJSON(t, "bundle item", item, NewBundleItemResponse(&item))
}

func TestSKUCostHistoryResponseMatchesDomainJSON(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	rec := domainSKU.SKUCostHistory{
		ID:            12,
		SKU:           "FD-RL-80-10",
		CostPrice:     42.5,
		EffectiveFrom: from,
		EffectiveTo:   &to,
		Note:          "ปรับต้นทุน",
		CreatedAt:     from,
		UpdatedAt:     from,
	}

	assertSameJSON(t, "cost history", rec, NewSKUCostHistoryResponse(&rec))
}

func TestLiveSessionResponseMatchesDomainJSON(t *testing.T) {
	now := time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC)
	approver := uint(9)
	session := domainLive.LiveSession{
		ID:               3,
		SessionNo:        "LS-20261103-001",
		StaffID:          9,
		Staff:            &domainAuth.User{ID: 9, Email: "a@b.c", Name: "Ann", Role: "live", IsActive: true, CreatedAt: now, UpdatedAt: now},
		LiveDate:         "2026-11-03",
		Platform:         domainLive.PlatformTikTok,
		TiktokAccount:    "@chawy",
		StartDatetime:    now,
		EndDatetime:      now.Add(2 * time.Hour),
		BreakMinutes:     15,
		RevenueGenerated: 12345.5,
		HasClip:          true,
		ClipLink:         "https://example.com/clip",
		LiveSummaryImage: "https://example.com/sum.png",
		HostNotes:        "good",
		Status:           domainLive.StatusApproved,
		ApprovedBy:       &approver,
		RejectionReason:  "",
		CreatedBy:        "owner",
		UpdatedBy:        "owner",
		AuditTrail:       "[]",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	assertSameJSON(t, "live session", session, NewLiveSessionResponse(&session))

	// nil staff must stay omitted exactly like the entity's omitempty.
	session.Staff = nil
	assertSameJSON(t, "live session (no staff)", session, NewLiveSessionResponse(&session))
}

func TestContentItemResponseMatchesDomainJSON(t *testing.T) {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	hostID := uint(4)
	sched := now.Add(48 * time.Hour)
	item := domainLive.ContentItem{
		ID:            5,
		Kind:          domainLive.ContentKindScheduled,
		Title:         "Live 11.11",
		Platform:      "TIKTOK",
		ScheduledFor:  &sched,
		PublishedAt:   nil,
		HostID:        &hostID,
		Host:          &domainAuth.User{ID: 4, Email: "h@b.c", Name: "Host", Role: "live", IsActive: true},
		HostName:      "Host",
		Reach:         1500,
		EngagementPct: 3.25,
		Notes:         "n",
		CreatedBy:     "staff",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	assertSameJSON(t, "content item", item, NewContentItemResponse(&item))
}

func TestSalesReturnResponseMatchesDomainJSON(t *testing.T) {
	now := time.Date(2026, 11, 2, 14, 30, 0, 0, time.UTC)
	orderID := uint(11)
	customerID := uint(22)
	line := domainReturn.SalesReturnLine{
		ID:             1,
		ReturnID:       8,
		SKU:            "FD-TN-80-01",
		Name:           "อกไก่",
		OrderedQty:     5,
		Quantity:       2,
		UnitPrice:      100,
		LineAmount:     200,
		Condition:      domainReturn.ConditionGood,
		Restock:        true,
		ReasonCode:     domainReturn.ReasonCustomerChange,
		EvidenceImages: []string{"https://example.com/1.jpg"},
		LotRef:         "LOT-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	ret := domainReturn.SalesReturn{
		ID:                 8,
		ReturnNo:           "SR-20261102-001",
		ReturnType:         domainReturn.ReturnTypeCustomer,
		OrderID:            &orderID,
		OrderNo:            "SO-001",
		InvoiceID:          nil,
		InvoiceNo:          "",
		CustomerID:         &customerID,
		CustomerName:       "ลูกค้า",
		Channel:            "tiktok",
		WarehouseID:        1,
		ReturnDate:         now,
		Status:             domainReturn.StatusApproved,
		Reason:             "เปลี่ยนใจ",
		Note:               "n",
		CancellationReason: "",
		TotalQty:           2,
		Subtotal:           200,
		VatAdjust:          14,
		NetAmount:          214,
		CreatedBy:          "staff",
		ApprovedBy:         "owner",
		CompletedBy:        "",
		Lines:              []domainReturn.SalesReturnLine{line},
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	assertSameJSON(t, "sales return", ret, NewSalesReturnResponse(&ret))

	// nil lines must stay JSON null exactly like the entity.
	ret.Lines = nil
	assertSameJSON(t, "sales return (no lines)", ret, NewSalesReturnResponse(&ret))
}

func TestJournalEntryResponseMatchesDomainJSON(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	reversal := uint(3)
	entry := domainFinance.JournalEntry{
		ID:           4,
		Code:         "JE-20261103-0001",
		Date:         "2026-11-03",
		SourceType:   "ORDER",
		SourceID:     77,
		SourceRef:    "SO-001",
		Description:  "ขายสินค้า",
		Status:       domainFinance.JournalStatusPosted,
		ReversalOfID: &reversal,
		CreatedBy:    "system",
		PostedAt:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
		Lines: []domainFinance.JournalLine{
			{
				ID:             1,
				JournalEntryID: 4,
				AccountID:      2,
				AccountCode:    "1100",
				AccountName:    "ลูกหนี้การค้า",
				Debit:          500,
				Credit:         0,
				SKU:            "FD-RL-80-10",
				Lot:            "LOT-1",
				Channel:        "tiktok",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}

	assertSameJSON(t, "journal entry", entry, NewJournalEntryResponse(&entry))

	entry.Lines = nil
	assertSameJSON(t, "journal entry (no lines)", entry, NewJournalEntryResponse(&entry))
}

func TestExpenseResponseMatchesDomainJSON(t *testing.T) {
	now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	exp := domainFinance.Expense{
		ID:          11,
		Code:        "EXP-20261030-0001",
		Date:        "2026-10-30",
		Category:    domainFinance.ExpenseCategoryAds,
		Channel:     domainFinance.ExpenseChannelTikTok,
		Amount:      1500.75,
		ProductName: "ยิงแอด",
		PurchaseURL: "https://example.com/buy",
		SlipURL:     "/uploads/expenses/a.jpg",
		Vendor:      "Meta",
		InvoiceRef:  "INV-1",
		Description: "โฆษณา",
		CreatedBy:   "owner",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	assertSameJSON(t, "expense", exp, NewExpenseResponse(&exp))
}

func TestShopeeOrderResponseMatchesDomainJSON(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	order := domainShopee.ShopeeOrder{
		ID:            "260301ABC01",
		OrderDate:     now,
		BuyerUsername: "buyer01",
		Province:      "กรุงเทพ",
		Status:        "COMPLETED",
		RowHash:       "abc123",
		Items: []domainShopee.ShopeeOrderItem{
			{
				ID:           1,
				OrderID:      "260301ABC01",
				SKU:          "FD-RL-80-10",
				ProductName:  "อกไก่",
				Qty:          2,
				SalePrice:    250,
				OriginalSKU:  "OLD-1",
				SKUConfirmed: true,
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assertSameJSON(t, "shopee order", order, NewShopeeOrderResponse(&order))

	order.Items = nil
	assertSameJSON(t, "shopee order (no items)", order, NewShopeeOrderResponse(&order))
}

func TestShopeeIncomeResponsesMatchDomainJSON(t *testing.T) {
	now := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	orderDate := now.Add(-48 * time.Hour)
	inc := domainShopee.ShopeeIncome{
		ID:           9,
		OrderID:      "260301ABC01",
		OrderDate:    &orderDate,
		TransferDate: now,
		NetAmount:    231.5,
		RowHash:      "def456",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assertSameJSON(t, "shopee income", inc, NewShopeeIncomeResponse(&inc))

	// The list item response must flatten the income exactly like the usecase
	// wrapper does (embedded struct + is_matched).
	listItem := struct {
		domainShopee.ShopeeIncome
		IsMatched bool `json:"is_matched"`
	}{ShopeeIncome: inc, IsMatched: true}

	assertSameJSON(t, "shopee income list item", listItem, ShopeeIncomeListItemResponse{
		ShopeeIncomeResponse: NewShopeeIncomeResponse(&inc),
		IsMatched:            true,
	})
}

func TestShopeeMatchingRowAndSummaryMatchDomainJSON(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	row := domainShopee.MatchingItemRow{
		ItemID:        3,
		OrderID:       "260301ABC01",
		OrderDate:     now,
		TransferDate:  now,
		SKU:           "FD-RL-80-10",
		ProductName:   "อกไก่",
		Qty:           2,
		EffectiveCost: 40,
		TotalCost:     80,
		LineSale:      250,
		OrderGross:    250,
		OrderNet:      231.5,
		AllocatedFee:  18.5,
		AllocatedNet:  231.5,
		Profit:        151.5,
		MarginPct:     60.6,
		Status:        "OK",
	}
	assertSameJSON(t, "shopee matching row", row, NewShopeeMatchingItemRowResponse(&row))

	summary := domainShopee.MonthlySummary{
		Month:           "2026-11",
		OrderCount:      10,
		ItemCount:       22,
		GrossSale:       5000,
		PlatformFees:    300,
		NetReceive:      4700,
		TotalCost:       2000,
		NetProfit:       2700,
		ProfitMarginPct: 54,
	}
	assertSameJSON(t, "shopee monthly summary", summary, NewShopeeMonthlySummaryResponse(&summary))

	stat := domainShopee.TopSKUStat{
		SKU:         "FD-RL-80-10",
		ProductName: "อกไก่",
		TotalQty:    20,
		TotalGross:  2500,
		TotalCost:   800,
		NetProfit:   1700,
		MarginPct:   68,
	}
	assertSameJSON(t, "shopee top sku", stat, NewShopeeTopSKUStatResponse(&stat))
}

func TestTikTokOrderResponseMatchesDomainJSON(t *testing.T) {
	order := domainTikTok.TiktokOrder{
		ID:                   "5770000000000000001",
		Date:                 "2026-11-03",
		Product:              "อกไก่",
		SKU:                  "FD-RL-80-10",
		Qty:                  2,
		Amount:               250,
		Status:               "COMPLETED",
		StockDeducted:        true,
		Imported:             true,
		NetRevenue:           231.5,
		PlatformFee:          18.5,
		Settled:              true,
		SettlementRef:        "STL-1",
		RecipientProvinceRaw: "Bangkok",
		RecipientProvince:    "กรุงเทพมหานคร",
		RecipientPostalCode:  "10110",
		Items: []domainTikTok.TiktokOrderItem{
			{
				ID:          1,
				OrderID:     "5770000000000000001",
				LineItemID:  "LI-1",
				ProductName: "อกไก่",
				SKU:         "FD-RL-80-10",
				Qty:         2,
				UnitPrice:   125,
				Amount:      250,
			},
		},
	}

	assertSameJSON(t, "tiktok order", order, NewTikTokOrderResponse(&order))

	// Zero-valued omitempty fields and nil items must serialize identically.
	bare := domainTikTok.TiktokOrder{ID: "x", Items: nil}
	assertSameJSON(t, "tiktok order (bare)", bare, NewTikTokOrderResponse(&bare))
}

func TestTikTokSyncRunAndLogMatchDomainJSON(t *testing.T) {
	started := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	finished := started.Add(37 * time.Second)
	run := domainTikTok.TiktokSyncRun{
		ID:            4,
		StartedAt:     started,
		FinishedAt:    &finished,
		Status:        "SUCCESS",
		Days:          30,
		Synced:        12,
		StockDeducted: 10,
	}
	assertSameJSON(t, "tiktok sync run", run, NewTikTokSyncRunResponse(&run))

	open := domainTikTok.TiktokSyncRun{ID: 5, StartedAt: started, Status: "RUNNING"}
	assertSameJSON(t, "tiktok sync run (open)", open, NewTikTokSyncRunResponse(&open))

	log := domainTikTok.SyncLog{
		ID:        2,
		OrderNo:   "5770000000000000001",
		Status:    "OK",
		Message:   "synced",
		CreatedAt: started,
	}
	assertSameJSON(t, "tiktok sync log", log, NewTikTokSyncLogResponse(&log))
}

func TestTikTokSKUMappingResponseMatchesLegacyJSON(t *testing.T) {
	created := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	m := domainTikTok.SKUMapping{
		ID:        3,
		TikTokSKU: "TT-001",
		LocalSKU:  "FD-RL-80-10",
		CreatedAt: created,
		UpdatedAt: created,
	}

	legacy := map[string]interface{}{
		"id":        3,
		"tiktokSku": "TT-001",
		"erpSku":    "FD-RL-80-10",
		"ratio":     1,
		"createdAt": "2026-11-03 08:00",
	}

	assertSameJSON(t, "tiktok sku mapping", legacy, NewTikTokSKUMappingResponse(&m))
}
