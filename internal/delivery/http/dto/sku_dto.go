package dto

type CreateSKURequest struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Barcode   string  `json:"barcode"`
	Category  string  `json:"category"`
	Flavor    string  `json:"flavor"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	IsBundle        bool    `json:"is_bundle"`
	Image           string  `json:"image"`
	InitialQuantity int     `json:"initial_quantity"`
}

// UpdateSKURequest uses pointers so omitted fields keep their stored values
// instead of being zeroed (FULL-11).
type UpdateSKURequest struct {
	Name      *string  `json:"name"`
	Barcode   *string  `json:"barcode"`
	Category  *string  `json:"category"`
	Flavor    *string  `json:"flavor"`
	Price     *float64 `json:"price"`
	CostPrice *float64 `json:"cost_price"`
	IsBundle  *bool    `json:"is_bundle"`
	Image     *string  `json:"image"`
	Status    *string  `json:"status"`
}

type SKUResponse struct {
	ID             uint       `json:"id"`
	SKU            string     `json:"sku"`
	Name           string     `json:"name"`
	Barcode        string     `json:"barcode"`
	Category       string     `json:"category"`
	Flavor         string     `json:"flavor"`
	Price          float64    `json:"price"`
	CostPrice      float64    `json:"cost_price"`
	IsBundle       bool       `json:"is_bundle"`
	Image          string     `json:"image"`
	Status         string     `json:"status"`
	CreatedAt      string     `json:"createdAt"`
	LastReceivedAt *string    `json:"lastReceivedAt"`
	ReceiptCount   int        `json:"receiptCount"`
}

type SKUReceiptResponse struct {
	ID               uint    `json:"id"`
	ReceivedAt       string  `json:"receivedAt"`
	SourceType       string  `json:"sourceType"`
	Quantity         int     `json:"quantity"`
	UnitCost         float64 `json:"unitCost"`
	RetailPrice      float64 `json:"retailPrice"`
	WarehouseID      uint    `json:"warehouseId"`
	WarehouseName    string  `json:"warehouseName"`
	LotNumber        string  `json:"lotNumber,omitempty"`
	SupplierLot      string  `json:"supplierLot,omitempty"`
	ExpiryDate       *string `json:"expiryDate,omitempty"`
	ReferenceType    string  `json:"referenceType"`
	ReferenceID      string  `json:"referenceId"`
	PurchaseOrderRef string  `json:"purchaseOrderRef,omitempty"`
	Note             string  `json:"note,omitempty"`
}


// BulkDeleteProductsRequest is the body of POST /products/bulk-delete.
type BulkDeleteProductsRequest struct {
	IDs []uint `json:"ids"`
}

// BulkDeleteProductItemResponse is the per-product outcome of a bulk delete.
type BulkDeleteProductItemResponse struct {
	ID      uint   `json:"id"`
	SKU     string `json:"sku"`
	Name    string `json:"name"`
	Deleted bool   `json:"deleted"`
	Code    string `json:"code,omitempty"`
	Error   string `json:"error,omitempty"`
}

// BulkDeleteProductsResponse summarizes a bulk delete.
type BulkDeleteProductsResponse struct {
	Requested int                             `json:"requested"`
	Deleted   int                             `json:"deleted"`
	Failed    int                             `json:"failed"`
	Results   []BulkDeleteProductItemResponse `json:"results"`
}
