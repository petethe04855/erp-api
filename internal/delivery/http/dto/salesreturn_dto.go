package dto

import (
	domainReturn "chawy-erp-api/internal/domain/salesreturn"
)

type CreateReturnLineRequest struct {
	SKU        string                     `json:"sku" validate:"required"`
	Quantity   int                        `json:"quantity" validate:"required,gt=0"`
	Condition  domainReturn.ItemCondition `json:"condition"`
	Restock    *bool                      `json:"restock"`
	ReasonCode domainReturn.ReasonCode    `json:"reason_code"`
	LotRef     string                     `json:"lot_ref"`
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
}

type CompleteReturnRequest struct {
	Lines []CompleteReturnLineRequest `json:"lines" validate:"required,min=1,dive"`
}

type ReasonRequest struct {
	Reason string `json:"reason" validate:"required"`
}
