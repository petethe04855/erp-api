package handlers

import "testing"

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
