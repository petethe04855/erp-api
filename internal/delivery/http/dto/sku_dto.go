package dto

type CreateSKURequest struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Barcode   string  `json:"barcode"`
	Category  string  `json:"category"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	IsBundle  bool    `json:"is_bundle"`
	Image     string  `json:"image"`
}

// UpdateSKURequest uses pointers so omitted fields keep their stored values
// instead of being zeroed (FULL-11).
type UpdateSKURequest struct {
	Name      *string  `json:"name"`
	Barcode   *string  `json:"barcode"`
	Category  *string  `json:"category"`
	Price     *float64 `json:"price"`
	CostPrice *float64 `json:"cost_price"`
	IsBundle  *bool    `json:"is_bundle"`
	Image     *string  `json:"image"`
	Status    *string  `json:"status"`
}

type SKUResponse struct {
	ID        uint    `json:"id"`
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Barcode   string  `json:"barcode"`
	Category  string  `json:"category"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	IsBundle  bool    `json:"is_bundle"`
	Image     string  `json:"image"`
	Status    string  `json:"status"`
}
