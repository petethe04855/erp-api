package handler

import "testing"

// Stock-out counts and booth bills are OUT movements, so they show in the
// goods-issue history; their channel must read as what they are.
func TestGoodsIssueChannelNamesCountsAndBooth(t *testing.T) {
	cases := map[string]string{
		"STOCK_OUT":  "นับของออก",
		"BOOTH_SALE": "ขายหน้าบูธ",
		"ORDER":      "Manual",
		"":           "Manual",
	}
	for ref, want := range cases {
		if got := formatMovementChannel(ref, ""); got != want {
			t.Errorf("formatMovementChannel(%q) = %q, want %q", ref, got, want)
		}
	}
}
