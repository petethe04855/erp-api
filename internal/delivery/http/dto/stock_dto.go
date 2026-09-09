package dto

type AdjustStockRequest struct {
	SKUID         uint   `json:"sku_id"`
	SKUCode       string `json:"sku_code"`
	WarehouseID   uint   `json:"warehouse_id"`
	Type          string `json:"type"` // IN, OUT, ADJUST
	Quantity      int    `json:"quantity"`
	ReferenceType string `json:"reference_type"`
	ReferenceID   string `json:"reference_id"`
	Note          string `json:"note"`
}

type StockResponse struct {
	ID           uint   `json:"id"`
	SKUID        uint   `json:"sku_id"`
	SKUCode      string `json:"sku_code"`
	WarehouseID  uint   `json:"warehouse_id"`
	Quantity     int    `json:"quantity"`
	ReservedQty  int    `json:"reserved_qty"`
	AvailableQty int    `json:"available_qty"`
}
