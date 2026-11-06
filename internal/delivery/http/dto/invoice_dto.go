package dto

import (
	"time"

	domainInvoice "chawy-erp-api/internal/domain/invoice"
)

type CreateInvoiceRequest struct {
	OrderID uint `json:"order_id"`
}

type MarkPaidRequest struct {
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	Method        string  `json:"method"`
	AccountCode   string  `json:"account_code"`
	AccountCode2  string  `json:"accountCode"`
}

type InvoiceResponse struct {
	ID            uint     `json:"id"`
	InvoiceNo     string   `json:"invoice_no"`
	OrderID       *uint    `json:"order_id,omitempty"`
	OrderNo       string   `json:"order_no,omitempty"`
	CustomerID    uint     `json:"customer_id"`
	CustomerName  string   `json:"customer_name"`
	Amount        float64  `json:"amount"`
	PaidAmount    float64  `json:"paid_amount"`
	Status        string   `json:"status"`
	PaymentMethod string   `json:"payment_method,omitempty"`
	DueDate       *string  `json:"due_date,omitempty"`
	PaidAt        *string  `json:"paid_at,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

func NewInvoiceResponse(inv *domainInvoice.Invoice) InvoiceResponse {
	if inv == nil {
		return InvoiceResponse{}
	}
	resp := InvoiceResponse{
		ID:            inv.ID,
		InvoiceNo:     inv.InvoiceNo,
		OrderID:       inv.OrderID,
		OrderNo:       inv.OrderNo,
		CustomerID:    inv.CustomerID,
		CustomerName:  inv.CustomerName,
		Amount:        inv.Amount,
		PaidAmount:    inv.PaidAmount,
		Status:        string(inv.Status),
		PaymentMethod: inv.PaymentMethod,
	}
	if inv.DueDate != nil && !inv.DueDate.IsZero() {
		d := inv.DueDate.Format(time.RFC3339)
		resp.DueDate = &d
	}
	if inv.PaidAt != nil && !inv.PaidAt.IsZero() {
		p := inv.PaidAt.Format(time.RFC3339)
		resp.PaidAt = &p
	}
	if !inv.CreatedAt.IsZero() {
		resp.CreatedAt = inv.CreatedAt.Format(time.RFC3339)
	}
	if !inv.UpdatedAt.IsZero() {
		resp.UpdatedAt = inv.UpdatedAt.Format(time.RFC3339)
	}
	return resp
}

func NewInvoiceResponses(items []domainInvoice.Invoice) []InvoiceResponse {
	out := make([]InvoiceResponse, len(items))
	for i := range items {
		out[i] = NewInvoiceResponse(&items[i])
	}
	return out
}
