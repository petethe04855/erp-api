package handlers

import (
	"testing"

	"chawy-erp-api/models"
)

func TestSummarizeAccountAmountsUsesLedgerSigns(t *testing.T) {
	summary := summarizeAccountAmounts(map[string]float64{
		"4000": 1000, "5100": -100, "5000": -400, "5200": -20, "6000": -80,
	})
	want := map[string]float64{"revenue": 900, "salesReturns": 100, "cogs": 400, "grossProfit": 500, "netProfit": 400}
	for key, expected := range want {
		if actual := summary[key].(float64); actual != expected {
			t.Fatalf("%s = %.2f, want %.2f", key, actual, expected)
		}
	}
}

func TestBuildRevenueReportIncludesSupportedChannelsOnly(t *testing.T) {
	report := buildRevenueReport([]models.SalesOrder{
		{Code: "SO-1", Customer: "Manual customer", Date: "2026-09-01", Channel: "Manual", Amount: 100},
		{Code: "SO-2", Customer: "Shopee customer", Date: "2026-09-02", Channel: "shopee", Amount: 200},
		{Code: "SO-3", Customer: "TikTok customer", Date: "2026-09-03", Channel: "Tik Tok", Amount: 300},
		{Code: "SO-4", Customer: "LINE customer", Date: "2026-09-04", Channel: "LINE", Amount: 400},
	})
	if report.Total != 600 || len(report.Rows) != 3 {
		t.Fatalf("total = %.2f and rows = %d, want 600.00 and 3", report.Total, len(report.Rows))
	}
	if report.ByChannel["Manual"] != 100 || report.ByChannel["Shopee"] != 200 || report.ByChannel["TikTok"] != 300 {
		t.Fatalf("unexpected channel totals: %#v", report.ByChannel)
	}
}
