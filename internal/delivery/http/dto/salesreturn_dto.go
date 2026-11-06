package dto

import (
	"time"

	domainReturn "chawy-erp-api/internal/domain/salesreturn"
)

type CreateReturnLineRequest struct {
	SKU        string                     `json:"sku" validate:"required"`
	Quantity   int                        `json:"quantity" validate:"required,gt=0"`
	Condition  domainReturn.ItemCondition `json:"condition"`
	Restock    *bool                      `json:"restock"`
	ReasonCode domainReturn.ReasonCode    `json:"reason_code"`
	// EvidenceImages = URLs รูปยืนยันสภาพสินค้า (จาก POST /upload/image)
	EvidenceImages []string `json:"evidence_images"`
	LotRef         string   `json:"lot_ref"`
}

type CreateReturnRequest struct {
	ReturnType  domainReturn.ReturnType   `json:"return_type"`
	OrderID     *uint                     `json:"order_id"`
	WarehouseID uint                      `json:"warehouse_id"`
	ReturnDate  string                    `json:"return_date"`
	Reason      string                    `json:"reason"`
	Note        string                    `json:"note"`
	Lines       []CreateReturnLineRequest `json:"lines" validate:"required,min=1,dive"`
}

type UpdateReturnRequest struct {
	WarehouseID uint                      `json:"warehouse_id"`
	ReturnDate  string                    `json:"return_date"`
	Reason      string                    `json:"reason"`
	Note        string                    `json:"note"`
	Lines       []CreateReturnLineRequest `json:"lines" validate:"required,min=1,dive"`
}

type CompleteReturnLineRequest struct {
	LineID    uint                       `json:"line_id" validate:"required"`
	Condition domainReturn.ItemCondition `json:"condition" validate:"required"`
	Restock   bool                       `json:"restock"`
	// AddEvidenceImages = รูปเพิ่มเติมจากคลังตอนตรวจรับ (ผลการตรวจสภาพจริง)
	AddEvidenceImages []string `json:"add_evidence_images"`
}

type CompleteReturnRequest struct {
	Lines []CompleteReturnLineRequest `json:"lines" validate:"required,min=1,dive"`
}

type ReasonRequest struct {
	Reason string `json:"reason" validate:"required"`
}

// SalesReturnLineResponse is the API contract for a returned line item.
type SalesReturnLineResponse struct {
	ID             uint                        `json:"id"`
	ReturnID       uint                        `json:"return_id"`
	SKU            string                      `json:"sku"`
	Name           string                      `json:"name"`
	OrderedQty     int                         `json:"ordered_qty"`
	Quantity       int                         `json:"quantity"`
	UnitPrice      float64                     `json:"unit_price"`
	LineAmount     float64                     `json:"line_amount"`
	Condition      domainReturn.ItemCondition  `json:"condition"`
	Restock        bool                        `json:"restock"`
	ReasonCode     domainReturn.ReasonCode     `json:"reason_code"`
	EvidenceImages []string                    `json:"evidence_images"`
	LotRef         string                      `json:"lot_ref"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

// SalesReturnResponse is the API contract for a sales return document.
type SalesReturnResponse struct {
	ID                 uint                      `json:"id"`
	ReturnNo           string                    `json:"return_no"`
	ReturnType         domainReturn.ReturnType   `json:"return_type"`
	OrderID            *uint                     `json:"order_id"`
	OrderNo            string                    `json:"order_no"`
	InvoiceID          *uint                     `json:"invoice_id"`
	InvoiceNo          string                    `json:"invoice_no"`
	CustomerID         *uint                     `json:"customer_id"`
	CustomerName       string                    `json:"customer_name"`
	Channel            string                    `json:"channel"`
	WarehouseID        uint                      `json:"warehouse_id"`
	ReturnDate         time.Time                 `json:"return_date"`
	Status             domainReturn.Status       `json:"status"`
	Reason             string                    `json:"reason"`
	Note               string                    `json:"note"`
	CancellationReason string                    `json:"cancellation_reason"`
	TotalQty           int                       `json:"total_qty"`
	Subtotal           float64                   `json:"subtotal"`
	VatAdjust          float64                   `json:"vat_adjust"`
	NetAmount          float64                   `json:"net_amount"`
	CreatedBy          string                    `json:"created_by"`
	ApprovedBy         string                    `json:"approved_by"`
	CompletedBy        string                    `json:"completed_by"`
	Lines              []SalesReturnLineResponse `json:"lines"`
	CreatedAt          time.Time                 `json:"created_at"`
	UpdatedAt          time.Time                 `json:"updated_at"`
}

// NewSalesReturnResponse maps a domain sales return to the API contract while
// keeping the previously published JSON shape byte-for-byte compatible.
func NewSalesReturnResponse(ret *domainReturn.SalesReturn) SalesReturnResponse {
	resp := SalesReturnResponse{
		ID:                 ret.ID,
		ReturnNo:           ret.ReturnNo,
		ReturnType:         ret.ReturnType,
		OrderID:            ret.OrderID,
		OrderNo:            ret.OrderNo,
		InvoiceID:          ret.InvoiceID,
		InvoiceNo:          ret.InvoiceNo,
		CustomerID:         ret.CustomerID,
		CustomerName:       ret.CustomerName,
		Channel:            ret.Channel,
		WarehouseID:        ret.WarehouseID,
		ReturnDate:         ret.ReturnDate,
		Status:             ret.Status,
		Reason:             ret.Reason,
		Note:               ret.Note,
		CancellationReason: ret.CancellationReason,
		TotalQty:           ret.TotalQty,
		Subtotal:           ret.Subtotal,
		VatAdjust:          ret.VatAdjust,
		NetAmount:          ret.NetAmount,
		CreatedBy:          ret.CreatedBy,
		ApprovedBy:         ret.ApprovedBy,
		CompletedBy:        ret.CompletedBy,
		CreatedAt:          ret.CreatedAt,
		UpdatedAt:          ret.UpdatedAt,
	}
	if ret.Lines != nil {
		resp.Lines = make([]SalesReturnLineResponse, len(ret.Lines))
		for i, l := range ret.Lines {
			resp.Lines[i] = SalesReturnLineResponse{
				ID:             l.ID,
				ReturnID:       l.ReturnID,
				SKU:            l.SKU,
				Name:           l.Name,
				OrderedQty:     l.OrderedQty,
				Quantity:       l.Quantity,
				UnitPrice:      l.UnitPrice,
				LineAmount:     l.LineAmount,
				Condition:      l.Condition,
				Restock:        l.Restock,
				ReasonCode:     l.ReasonCode,
				EvidenceImages: l.EvidenceImages,
				LotRef:         l.LotRef,
				CreatedAt:      l.CreatedAt,
				UpdatedAt:      l.UpdatedAt,
			}
		}
	}
	return resp
}

// NewSalesReturnResponses maps a slice of domain sales returns.
func NewSalesReturnResponses(items []domainReturn.SalesReturn) []SalesReturnResponse {
	out := make([]SalesReturnResponse, len(items))
	for i := range items {
		out[i] = NewSalesReturnResponse(&items[i])
	}
	return out
}
