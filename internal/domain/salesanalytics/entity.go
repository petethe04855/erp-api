// Package salesanalytics exposes the raw marketplace order facts that the CRM
// backoffice aggregates into its sales dashboard. It returns facts only: every
// KPI, share and ranking is computed in crm-core
// (plan: CRM_BACKOFFICE_MARKETING.md §6), so the ERP stays the source of truth
// for orders while the CRM stays the source of truth for the report.
package salesanalytics

import "context"

// Channel filters accepted by the endpoint.
const (
	ChannelAll    = "all"
	ChannelShopee = "shopee"
	ChannelTikTok = "tiktok"
)

// Query filters the order facts. Dates are inclusive calendar days in Bangkok
// time, formatted "2006-01-02".
type Query struct {
	DateFrom string
	DateTo   string
	Channel  string
	Region   string
}

// Line is one SKU line of an order.
type Line struct {
	SKU    string  `json:"sku"`
	Name   string  `json:"name"`
	Qty    int     `json:"qty"`
	Amount float64 `json:"amount"`
}

// Order is one marketplace order fact. It carries platform usernames only, no
// recipient names or addresses (PDPA). Fields the marketplace does not expose
// (TikTok buyer and flavor) stay empty rather than being guessed.
type Order struct {
	Platform    string   `json:"platform"`
	OrderNo     string   `json:"orderNo"`
	OrderedAt   string   `json:"orderedAt"`
	HasTime     bool     `json:"hasTime"`
	BuyerID     string   `json:"buyerId"`
	Date        string   `json:"date"`
	Amount      float64  `json:"amount"`
	Buyer       string   `json:"buyer"`
	Channel     string   `json:"channel"`
	ChannelType string   `json:"channelType"`
	Province    string   `json:"province"`
	Sets        []string `json:"sets"`
	Items       []string `json:"items"`
	Lines       []Line   `json:"lines"`
}

// Payload is the response body of GET /crm/sales-analytics.
type Payload struct {
	Orders []Order `json:"orders"`
}

// Repository reads order facts from the ERP database. It applies the date and
// channel filters; the usecase applies the region grouping.
type Repository interface {
	SalesOrders(ctx context.Context, q Query) ([]Order, error)
}
