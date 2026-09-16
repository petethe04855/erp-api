package dto

type CreateOrderItemDTO struct {
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type CreateOrderRequest struct {
	CustomerID   uint                 `json:"customer_id"`
	CustomerName string               `json:"customer_name"`
	Channel      string               `json:"channel"`
	Note         string               `json:"note"`
	Items        []CreateOrderItemDTO `json:"items"`
}

type ShipOrderRequest struct {
	WarehouseID uint `json:"warehouse_id"`
}
