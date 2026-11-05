package dto

import (
	"time"

	domainTikTok "chawy-erp-api/internal/domain/tiktok"
)

// TikTokOrderItemResponse is the API contract for a TikTok order line. Field
// names mirror the previous domain-entity output exactly.
type TikTokOrderItemResponse struct {
	ID          uint    `json:"id"`
	OrderID     string  `json:"orderId"`
	LineItemID  string  `json:"lineItemId"`
	ProductName string  `json:"productName"`
	SKU         string  `json:"sku"`
	Qty         int     `json:"qty"`
	UnitPrice   float64 `json:"unitPrice"`
	Amount      float64 `json:"amount"`
}

// NewTikTokOrderItemResponse maps a domain order item to the API contract.
func NewTikTokOrderItemResponse(it *domainTikTok.TiktokOrderItem) TikTokOrderItemResponse {
	return TikTokOrderItemResponse{
		ID:          it.ID,
		OrderID:     it.OrderID,
		LineItemID:  it.LineItemID,
		ProductName: it.ProductName,
		SKU:         it.SKU,
		Qty:         it.Qty,
		UnitPrice:   it.UnitPrice,
		Amount:      it.Amount,
	}
}

// NewTikTokOrderItemResponses maps a slice of domain order items.
func NewTikTokOrderItemResponses(items []domainTikTok.TiktokOrderItem) []TikTokOrderItemResponse {
	if items == nil {
		return nil
	}
	out := make([]TikTokOrderItemResponse, len(items))
	for i := range items {
		out[i] = NewTikTokOrderItemResponse(&items[i])
	}
	return out
}

// TikTokOrderResponse is the API contract for a TikTok order header.
type TikTokOrderResponse struct {
	ID                   string                    `json:"id"`
	Date                 string                    `json:"date"`
	Product              string                    `json:"product"`
	SKU                  string                    `json:"sku"`
	Qty                  int                       `json:"qty"`
	Amount               float64                   `json:"amount"`
	Status               string                    `json:"status"`
	StockDeducted        bool                      `json:"stockDeducted"`
	Imported             bool                      `json:"imported"`
	NetRevenue           float64                   `json:"netRevenue,omitempty"`
	PlatformFee          float64                   `json:"platformFee,omitempty"`
	Settled              bool                      `json:"settled"`
	SettlementRef        string                    `json:"settlementRef,omitempty"`
	RecipientProvinceRaw string                    `json:"recipientProvinceRaw"`
	RecipientProvince    string                    `json:"recipientProvince"`
	RecipientPostalCode  string                    `json:"recipientPostalCode"`
	Items                []TikTokOrderItemResponse `json:"items"`
}

// NewTikTokOrderResponse maps a domain order to the API contract.
func NewTikTokOrderResponse(o *domainTikTok.TiktokOrder) TikTokOrderResponse {
	return TikTokOrderResponse{
		ID:                   o.ID,
		Date:                 o.Date,
		Product:              o.Product,
		SKU:                  o.SKU,
		Qty:                  o.Qty,
		Amount:               o.Amount,
		Status:               o.Status,
		StockDeducted:        o.StockDeducted,
		Imported:             o.Imported,
		NetRevenue:           o.NetRevenue,
		PlatformFee:          o.PlatformFee,
		Settled:              o.Settled,
		SettlementRef:        o.SettlementRef,
		RecipientProvinceRaw: o.RecipientProvinceRaw,
		RecipientProvince:    o.RecipientProvince,
		RecipientPostalCode:  o.RecipientPostalCode,
		Items:                NewTikTokOrderItemResponses(o.Items),
	}
}

// NewTikTokOrderResponses maps a slice of domain orders.
func NewTikTokOrderResponses(items []domainTikTok.TiktokOrder) []TikTokOrderResponse {
	if items == nil {
		return nil
	}
	out := make([]TikTokOrderResponse, len(items))
	for i := range items {
		out[i] = NewTikTokOrderResponse(&items[i])
	}
	return out
}

// TikTokSyncRunResponse is the API contract for a sync history entry.
type TikTokSyncRunResponse struct {
	ID            uint       `json:"id"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	Status        string     `json:"status"`
	Days          int        `json:"days"`
	Synced        int        `json:"synced"`
	StockDeducted int        `json:"stockDeducted"`
	Error         string     `json:"error,omitempty"`
}

// NewTikTokSyncRunResponse maps a domain sync run to the API contract.
func NewTikTokSyncRunResponse(r *domainTikTok.TiktokSyncRun) TikTokSyncRunResponse {
	return TikTokSyncRunResponse{
		ID:            r.ID,
		StartedAt:     r.StartedAt,
		FinishedAt:    r.FinishedAt,
		Status:        r.Status,
		Days:          r.Days,
		Synced:        r.Synced,
		StockDeducted: r.StockDeducted,
		Error:         r.Error,
	}
}

// NewTikTokSyncRunResponses maps a slice of domain sync runs.
func NewTikTokSyncRunResponses(items []domainTikTok.TiktokSyncRun) []TikTokSyncRunResponse {
	if items == nil {
		return nil
	}
	out := make([]TikTokSyncRunResponse, len(items))
	for i := range items {
		out[i] = NewTikTokSyncRunResponse(&items[i])
	}
	return out
}

// TikTokSyncLogResponse is the API contract for a webhook/API sync log.
type TikTokSyncLogResponse struct {
	ID        uint      `json:"id"`
	OrderNo   string    `json:"order_no"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// NewTikTokSyncLogResponse maps a domain sync log to the API contract.
func NewTikTokSyncLogResponse(l *domainTikTok.SyncLog) TikTokSyncLogResponse {
	return TikTokSyncLogResponse{
		ID:        l.ID,
		OrderNo:   l.OrderNo,
		Status:    l.Status,
		Message:   l.Message,
		CreatedAt: l.CreatedAt,
	}
}

// NewTikTokSyncLogResponses maps a slice of domain sync logs.
func NewTikTokSyncLogResponses(items []domainTikTok.SyncLog) []TikTokSyncLogResponse {
	if items == nil {
		return nil
	}
	out := make([]TikTokSyncLogResponse, len(items))
	for i := range items {
		out[i] = NewTikTokSyncLogResponse(&items[i])
	}
	return out
}

// TikTokSKUMappingResponse is the API contract for a TikTok-to-ERP SKU binding.
// It keeps the camelCase shape the marketing UI already consumes.
type TikTokSKUMappingResponse struct {
	ID        uint   `json:"id"`
	TikTokSKU string `json:"tiktokSku"`
	ERPSKU    string `json:"erpSku"`
	Ratio     int    `json:"ratio"`
	CreatedAt string `json:"createdAt"`
}

// NewTikTokSKUMappingResponse maps a domain SKU mapping to the API contract.
func NewTikTokSKUMappingResponse(m *domainTikTok.SKUMapping) TikTokSKUMappingResponse {
	return TikTokSKUMappingResponse{
		ID:        m.ID,
		TikTokSKU: m.TikTokSKU,
		ERPSKU:    m.LocalSKU,
		Ratio:     1,
		CreatedAt: m.CreatedAt.Format("2006-01-02 15:04"),
	}
}

// NewTikTokSKUMappingResponses maps a slice of domain SKU mappings.
func NewTikTokSKUMappingResponses(items []domainTikTok.SKUMapping) []TikTokSKUMappingResponse {
	out := make([]TikTokSKUMappingResponse, len(items))
	for i := range items {
		out[i] = NewTikTokSKUMappingResponse(&items[i])
	}
	return out
}
