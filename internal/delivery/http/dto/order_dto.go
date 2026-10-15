package dto

import (
	"time"

	domainOrder "chawy-erp-api/internal/domain/order"
)

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

type OrderItemResponse struct {
	ID        uint    `json:"id"`
	OrderID   uint    `json:"order_id"`
	SKU       string  `json:"sku"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Subtotal  float64 `json:"subtotal"`
	CreatedAt string  `json:"created_at,omitempty"`
}

type OrderResponse struct {
	ID           uint                `json:"id"`
	OrderNo      string              `json:"order_no"`
	CustomerID   uint                `json:"customer_id"`
	CustomerName string              `json:"customer_name"`
	Channel      string              `json:"channel"`
	Status       string              `json:"status"`
	TotalAmount  float64             `json:"total_amount"`
	Note         string              `json:"note"`
	Items        []OrderItemResponse `json:"items"`
	CreatedAt    string              `json:"created_at,omitempty"`
	UpdatedAt    string              `json:"updated_at,omitempty"`
}

func NewOrderResponse(o *domainOrder.Order) OrderResponse {
	if o == nil {
		return OrderResponse{}
	}
	items := make([]OrderItemResponse, len(o.Items))
	for i, it := range o.Items {
		var itemCreatedAt string
		if !it.CreatedAt.IsZero() {
			itemCreatedAt = it.CreatedAt.Format(time.RFC3339)
		}
		items[i] = OrderItemResponse{
			ID:        it.ID,
			OrderID:   it.OrderID,
			SKU:       it.SKU,
			Name:      it.Name,
			Price:     it.Price,
			Quantity:  it.Quantity,
			Subtotal:  it.Subtotal,
			CreatedAt: itemCreatedAt,
		}
	}
	resp := OrderResponse{
		ID:           o.ID,
		OrderNo:      o.OrderNo,
		CustomerID:   o.CustomerID,
		CustomerName: o.CustomerName,
		Channel:      o.Channel,
		Status:       string(o.Status),
		TotalAmount:  o.TotalAmount,
		Note:         o.Note,
		Items:        items,
	}
	if !o.CreatedAt.IsZero() {
		resp.CreatedAt = o.CreatedAt.Format(time.RFC3339)
	}
	if !o.UpdatedAt.IsZero() {
		resp.UpdatedAt = o.UpdatedAt.Format(time.RFC3339)
	}
	return resp
}

func NewOrderResponses(items []domainOrder.Order) []OrderResponse {
	out := make([]OrderResponse, len(items))
	for i := range items {
		out[i] = NewOrderResponse(&items[i])
	}
	return out
}
