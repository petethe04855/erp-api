package dto

import (
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
)

// ShopeeOrderItemResponse is the API contract for an order line. Field names
// match the previous domain-entity output exactly (consumed by
// features/shopee/types/shopee.types.ts).
type ShopeeOrderItemResponse struct {
	ID           uint      `json:"id"`
	OrderID      string    `json:"order_id"`
	SKU          string    `json:"sku"`
	ProductName  string    `json:"product_name"`
	Qty          int       `json:"qty"`
	SalePrice    float64   `json:"sale_price"`
	OriginalSKU  string    `json:"original_sku"`
	SKUConfirmed bool      `json:"sku_confirmed"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewShopeeOrderItemResponse maps a domain order item to the API contract.
func NewShopeeOrderItemResponse(it *domainShopee.ShopeeOrderItem) ShopeeOrderItemResponse {
	return ShopeeOrderItemResponse{
		ID:           it.ID,
		OrderID:      it.OrderID,
		SKU:          it.SKU,
		ProductName:  it.ProductName,
		Qty:          it.Qty,
		SalePrice:    it.SalePrice,
		OriginalSKU:  it.OriginalSKU,
		SKUConfirmed: it.SKUConfirmed,
		CreatedAt:    it.CreatedAt,
		UpdatedAt:    it.UpdatedAt,
	}
}

// NewShopeeOrderItemResponses maps a slice of domain order items.
func NewShopeeOrderItemResponses(items []domainShopee.ShopeeOrderItem) []ShopeeOrderItemResponse {
	if items == nil {
		return nil
	}
	out := make([]ShopeeOrderItemResponse, len(items))
	for i := range items {
		out[i] = NewShopeeOrderItemResponse(&items[i])
	}
	return out
}

// ShopeeOrderResponse is the API contract for a Shopee order.
type ShopeeOrderResponse struct {
	ID            string                    `json:"id"`
	OrderDate     time.Time                 `json:"order_date"`
	BuyerUsername string                    `json:"buyer_username"`
	Province      string                    `json:"province"`
	Status        string                    `json:"status"`
	RowHash       string                    `json:"row_hash"`
	Items         []ShopeeOrderItemResponse `json:"items"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
}

// NewShopeeOrderResponse maps a domain order to the API contract.
func NewShopeeOrderResponse(o *domainShopee.ShopeeOrder) ShopeeOrderResponse {
	return ShopeeOrderResponse{
		ID:            o.ID,
		OrderDate:     o.OrderDate,
		BuyerUsername: o.BuyerUsername,
		Province:      o.Province,
		Status:        o.Status,
		RowHash:       o.RowHash,
		Items:         NewShopeeOrderItemResponses(o.Items),
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
	}
}

// NewShopeeOrderResponses maps a slice of domain orders.
func NewShopeeOrderResponses(items []domainShopee.ShopeeOrder) []ShopeeOrderResponse {
	if items == nil {
		return nil
	}
	out := make([]ShopeeOrderResponse, len(items))
	for i := range items {
		out[i] = NewShopeeOrderResponse(&items[i])
	}
	return out
}

// ShopeeIncomeResponse is the API contract for a Shopee settlement record.
type ShopeeIncomeResponse struct {
	ID           uint       `json:"id"`
	OrderID      string     `json:"order_id"`
	OrderDate    *time.Time `json:"order_date"`
	TransferDate time.Time  `json:"transfer_date"`
	NetAmount    float64    `json:"net_amount"`
	RowHash      string     `json:"row_hash"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// NewShopeeIncomeResponse maps a domain income record to the API contract.
func NewShopeeIncomeResponse(inc *domainShopee.ShopeeIncome) ShopeeIncomeResponse {
	return ShopeeIncomeResponse{
		ID:           inc.ID,
		OrderID:      inc.OrderID,
		OrderDate:    inc.OrderDate,
		TransferDate: inc.TransferDate,
		NetAmount:    inc.NetAmount,
		RowHash:      inc.RowHash,
		CreatedAt:    inc.CreatedAt,
		UpdatedAt:    inc.UpdatedAt,
	}
}

// NewShopeeIncomeResponses maps a slice of domain income records.
func NewShopeeIncomeResponses(items []domainShopee.ShopeeIncome) []ShopeeIncomeResponse {
	if items == nil {
		return nil
	}
	out := make([]ShopeeIncomeResponse, len(items))
	for i := range items {
		out[i] = NewShopeeIncomeResponse(&items[i])
	}
	return out
}

// ShopeeIncomeListItemResponse is an income record plus its match flag. The
// embedded response keeps the flattened JSON shape clients already consume.
type ShopeeIncomeListItemResponse struct {
	ShopeeIncomeResponse
	IsMatched bool `json:"is_matched"`
}

// ShopeeMatchingItemRowResponse is the API contract for one matching row.
type ShopeeMatchingItemRowResponse struct {
	ItemID        uint      `json:"item_id"`
	OrderID       string    `json:"order_id"`
	OrderDate     time.Time `json:"order_date"`
	TransferDate  time.Time `json:"transfer_date"`
	SKU           string    `json:"sku"`
	ProductName   string    `json:"product_name"`
	Qty           int       `json:"qty"`
	EffectiveCost float64   `json:"effective_cost"`
	TotalCost     float64   `json:"total_cost"`
	LineSale      float64   `json:"line_sale"`
	OrderGross    float64   `json:"order_gross"`
	OrderNet      float64   `json:"order_net"`
	AllocatedFee  float64   `json:"allocated_fee"`
	AllocatedNet  float64   `json:"allocated_net"`
	Profit        float64   `json:"profit"`
	MarginPct     float64   `json:"margin_pct"`
	Status        string    `json:"status"`
}

// NewShopeeMatchingItemRowResponse maps a domain matching row.
func NewShopeeMatchingItemRowResponse(r *domainShopee.MatchingItemRow) ShopeeMatchingItemRowResponse {
	return ShopeeMatchingItemRowResponse{
		ItemID:        r.ItemID,
		OrderID:       r.OrderID,
		OrderDate:     r.OrderDate,
		TransferDate:  r.TransferDate,
		SKU:           r.SKU,
		ProductName:   r.ProductName,
		Qty:           r.Qty,
		EffectiveCost: r.EffectiveCost,
		TotalCost:     r.TotalCost,
		LineSale:      r.LineSale,
		OrderGross:    r.OrderGross,
		OrderNet:      r.OrderNet,
		AllocatedFee:  r.AllocatedFee,
		AllocatedNet:  r.AllocatedNet,
		Profit:        r.Profit,
		MarginPct:     r.MarginPct,
		Status:        r.Status,
	}
}

// NewShopeeMatchingItemRowResponses maps a slice of domain matching rows.
func NewShopeeMatchingItemRowResponses(items []domainShopee.MatchingItemRow) []ShopeeMatchingItemRowResponse {
	if items == nil {
		return nil
	}
	out := make([]ShopeeMatchingItemRowResponse, len(items))
	for i := range items {
		out[i] = NewShopeeMatchingItemRowResponse(&items[i])
	}
	return out
}

// ShopeeMonthlySummaryResponse is the API contract for a monthly rollup.
type ShopeeMonthlySummaryResponse struct {
	Month           string  `json:"month"`
	OrderCount      int     `json:"order_count"`
	ItemCount       int     `json:"item_count"`
	GrossSale       float64 `json:"gross_sale"`
	PlatformFees    float64 `json:"platform_fees"`
	NetReceive      float64 `json:"net_receive"`
	TotalCost       float64 `json:"total_cost"`
	NetProfit       float64 `json:"net_profit"`
	ProfitMarginPct float64 `json:"profit_margin_pct"`
}

// NewShopeeMonthlySummaryResponse maps a domain monthly summary.
func NewShopeeMonthlySummaryResponse(s *domainShopee.MonthlySummary) ShopeeMonthlySummaryResponse {
	return ShopeeMonthlySummaryResponse{
		Month:           s.Month,
		OrderCount:      s.OrderCount,
		ItemCount:       s.ItemCount,
		GrossSale:       s.GrossSale,
		PlatformFees:    s.PlatformFees,
		NetReceive:      s.NetReceive,
		TotalCost:       s.TotalCost,
		NetProfit:       s.NetProfit,
		ProfitMarginPct: s.ProfitMarginPct,
	}
}

// NewShopeeMonthlySummaryResponses maps a slice of domain monthly summaries.
func NewShopeeMonthlySummaryResponses(items []domainShopee.MonthlySummary) []ShopeeMonthlySummaryResponse {
	if items == nil {
		return nil
	}
	out := make([]ShopeeMonthlySummaryResponse, len(items))
	for i := range items {
		out[i] = NewShopeeMonthlySummaryResponse(&items[i])
	}
	return out
}

// ShopeeTopSKUStatResponse is the API contract for an aggregate SKU stat.
type ShopeeTopSKUStatResponse struct {
	SKU         string  `json:"sku"`
	ProductName string  `json:"product_name"`
	TotalQty    int     `json:"total_qty"`
	TotalGross  float64 `json:"total_gross"`
	TotalCost   float64 `json:"total_cost"`
	NetProfit   float64 `json:"net_profit"`
	MarginPct   float64 `json:"margin_pct"`
}

// NewShopeeTopSKUStatResponse maps a domain top-SKU stat.
func NewShopeeTopSKUStatResponse(s *domainShopee.TopSKUStat) ShopeeTopSKUStatResponse {
	return ShopeeTopSKUStatResponse{
		SKU:         s.SKU,
		ProductName: s.ProductName,
		TotalQty:    s.TotalQty,
		TotalGross:  s.TotalGross,
		TotalCost:   s.TotalCost,
		NetProfit:   s.NetProfit,
		MarginPct:   s.MarginPct,
	}
}

// NewShopeeTopSKUStatResponses maps a slice of domain top-SKU stats.
func NewShopeeTopSKUStatResponses(items []domainShopee.TopSKUStat) []ShopeeTopSKUStatResponse {
	if items == nil {
		return nil
	}
	out := make([]ShopeeTopSKUStatResponse, len(items))
	for i := range items {
		out[i] = NewShopeeTopSKUStatResponse(&items[i])
	}
	return out
}

// ShopeeDuplicateDetailResponse mirrors the duplicate report row shape.
type ShopeeDuplicateDetailResponse struct {
	Identifier    string `json:"identifier"`
	RowIndex      int    `json:"row_index"`
	DuplicateType string `json:"duplicate_type"`
	Message       string `json:"message"`
}

// ShopeeOrderPreviewResponse is the API contract for the order-import preview.
type ShopeeOrderPreviewResponse struct {
	TotalRows        int                             `json:"total_rows"`
	TotalOrders      int                             `json:"total_orders"`
	BlankSKUCount    int                             `json:"blank_sku_count"`
	DuplicateCount   int                             `json:"duplicate_count"`
	DuplicateOrders  []string                        `json:"duplicate_orders"`
	DuplicateDetails []ShopeeDuplicateDetailResponse `json:"duplicate_details"`
	SampleRows       []ShopeeOrderItemResponse       `json:"sample_rows"`
	SampleOrders     []ShopeeOrderResponse           `json:"sample_orders"`
}

// ShopeeIncomePreviewResponse is the API contract for the income-import preview.
type ShopeeIncomePreviewResponse struct {
	TotalRows        int                             `json:"total_rows"`
	TotalNetAmount   float64                         `json:"total_net_amount"`
	DuplicateCount   int                             `json:"duplicate_count"`
	DuplicateOrders  []string                        `json:"duplicate_orders"`
	DuplicateDetails []ShopeeDuplicateDetailResponse `json:"duplicate_details"`
	SampleRows       []ShopeeIncomeResponse          `json:"sample_rows"`
}

// ShopeeDashboardResponse is the API contract for Shopee analytics.
type ShopeeDashboardResponse struct {
	FromMonth     string                          `json:"from_month"`
	ToMonth       string                          `json:"to_month"`
	YTDSummary    ShopeeMonthlySummaryResponse    `json:"ytd_summary"`
	MonthlyTrends []ShopeeMonthlySummaryResponse  `json:"monthly_trends"`
	TopSKUs       []ShopeeTopSKUStatResponse      `json:"top_skus"`
}
