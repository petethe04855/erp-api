package tiktok

import (
	"encoding/json"
	"testing"
)

func TestOrderSearchResponseReadsBuyerUserID(t *testing.T) {
	raw := `{"code":0,"message":"Success","data":{"next_page_token":"","total_count":1,"orders":[
		{"id":"5801","status":"DELIVERED","create_time":1763000000,"user_id":"7494000000000001",
		 "payment":{"total_amount":"99"},"line_items":[{"id":"L1","product_name":"ขนมไก่","seller_sku":"CHK-01","quantity":1,"sale_price":"99"}]}]}}`
	var resp OrderSearchResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if got := resp.Data.Orders[0].UserID; got != "7494000000000001" {
		t.Fatalf("UserID = %q", got)
	}
}
