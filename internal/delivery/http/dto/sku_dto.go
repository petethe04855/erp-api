package dto

type CreateSKURequest struct {
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Barcode   string  `json:"barcode"`
	Category  string  `json:"category"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	IsBundle  bool    `json:"is_bundle"`
}

type UpdateSKURequest struct {
	Name      string  `json:"name"`
	Barcode   string  `json:"barcode"`
	Category  string  `json:"category"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	IsBundle  bool    `json:"is_bundle"`
	Status    string  `json:"status"`
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
	Status    string  `json:"status"`
}
