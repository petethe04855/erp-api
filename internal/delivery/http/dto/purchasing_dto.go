package dto

import (
	"time"

	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
)

type CreateSupplierRequest struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	ContactPerson string `json:"contact_person"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Address       string `json:"address"`
}

type CreatePOItemDTO struct {
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	UnitCost float64 `json:"unit_cost"`
}

type CreatePORequest struct {
	SupplierID uint              `json:"supplier_id"`
	Note       string            `json:"note"`
	Items      []CreatePOItemDTO `json:"items"`
}

type ReceiveGoodsItemDTO struct {
	SKU         string `json:"sku"`
	Quantity    int    `json:"quantity"`
	LotNumber   string `json:"lot_number"`
	SupplierLot string `json:"supplier_lot"`
	ExpiryDate  string `json:"expiry_date"`
}

type ReceiveGoodsRequest struct {
	WarehouseID uint                  `json:"warehouse_id"`
	Items       []ReceiveGoodsItemDTO `json:"items"`
}

type SupplierResponse struct {
	ID            uint   `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	ContactPerson string `json:"contact_person"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Address       string `json:"address"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

func NewSupplierResponse(s *domainPurchasing.Supplier) SupplierResponse {
	if s == nil {
		return SupplierResponse{}
	}
	resp := SupplierResponse{
		ID:            s.ID,
		Code:          s.Code,
		Name:          s.Name,
		ContactPerson: s.ContactPerson,
		Phone:         s.Phone,
		Email:         s.Email,
		Address:       s.Address,
		Status:        s.Status,
	}
	if !s.CreatedAt.IsZero() {
		resp.CreatedAt = s.CreatedAt.Format(time.RFC3339)
	}
	if !s.UpdatedAt.IsZero() {
		resp.UpdatedAt = s.UpdatedAt.Format(time.RFC3339)
	}
	return resp
}

func NewSupplierResponses(items []domainPurchasing.Supplier) []SupplierResponse {
	out := make([]SupplierResponse, len(items))
	for i := range items {
		out[i] = NewSupplierResponse(&items[i])
	}
	return out
}

type POItemResponse struct {
	ID          uint    `json:"id"`
	POID        uint    `json:"po_id"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	UnitCost    float64 `json:"unit_cost"`
	Quantity    int     `json:"quantity"`
	ReceivedQty int     `json:"received_qty"`
	Subtotal    float64 `json:"subtotal"`
	CreatedAt   string  `json:"created_at,omitempty"`
}

type PurchaseOrderResponse struct {
	ID           uint             `json:"id"`
	PONo         string           `json:"po_no"`
	PONumber     string           `json:"poNumber,omitempty"`
	SupplierID   uint             `json:"supplier_id"`
	SupplierName string           `json:"supplier_name"`
	Status       string           `json:"status"`
	TotalAmount  float64          `json:"total_amount"`
	Note         string           `json:"note"`
	ETADate      string           `json:"eta_date,omitempty"`
	Items        []POItemResponse `json:"items"`
	CreatedAt    string           `json:"created_at,omitempty"`
	UpdatedAt    string           `json:"updated_at,omitempty"`
}

func NewPurchaseOrderResponse(po *domainPurchasing.PurchaseOrder) PurchaseOrderResponse {
	if po == nil {
		return PurchaseOrderResponse{}
	}
	items := make([]POItemResponse, len(po.Items))
	for i, it := range po.Items {
		var itCreatedAt string
		if !it.CreatedAt.IsZero() {
			itCreatedAt = it.CreatedAt.Format(time.RFC3339)
		}
		items[i] = POItemResponse{
			ID:          it.ID,
			POID:        it.POID,
			SKU:         it.SKU,
			Name:        it.Name,
			UnitCost:    it.UnitCost,
			Quantity:    it.Quantity,
			ReceivedQty: it.ReceivedQty,
			Subtotal:    it.Subtotal,
			CreatedAt:   itCreatedAt,
		}
	}
	resp := PurchaseOrderResponse{
		ID:           po.ID,
		PONo:         po.PONo,
		PONumber:     po.PONo,
		SupplierID:   po.SupplierID,
		SupplierName: po.SupplierName,
		Status:       string(po.Status),
		TotalAmount:  po.TotalAmount,
		Note:         po.Note,
		ETADate:      po.ETADate,
		Items:        items,
	}
	if !po.CreatedAt.IsZero() {
		resp.CreatedAt = po.CreatedAt.Format(time.RFC3339)
	}
	if !po.UpdatedAt.IsZero() {
		resp.UpdatedAt = po.UpdatedAt.Format(time.RFC3339)
	}
	return resp
}

func NewPurchaseOrderResponses(items []domainPurchasing.PurchaseOrder) []PurchaseOrderResponse {
	out := make([]PurchaseOrderResponse, len(items))
	for i := range items {
		out[i] = NewPurchaseOrderResponse(&items[i])
	}
	return out
}

