package dto

import (
	"time"

	domainSKU "chawy-erp-api/internal/domain/sku"
)

// SKUCostHistoryResponse is the API contract for SKU cost history. It mirrors
// the domain entity's JSON field names (snake_case, consumed by
// features/shopee and SKUCostHistoryModal) so the domain entity never leaves
// the delivery layer as-is.
type SKUCostHistoryResponse struct {
	ID            uint       `json:"id"`
	SKU           string     `json:"sku"`
	CostPrice     float64    `json:"cost_price"`
	EffectiveFrom time.Time  `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to"`
	Note          string     `json:"note"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// NewSKUCostHistoryResponse maps a domain cost history record to the contract.
func NewSKUCostHistoryResponse(h *domainSKU.SKUCostHistory) SKUCostHistoryResponse {
	return SKUCostHistoryResponse{
		ID:            h.ID,
		SKU:           h.SKU,
		CostPrice:     h.CostPrice,
		EffectiveFrom: h.EffectiveFrom,
		EffectiveTo:   h.EffectiveTo,
		Note:          h.Note,
		CreatedAt:     h.CreatedAt,
		UpdatedAt:     h.UpdatedAt,
	}
}

// NewSKUCostHistoryResponses maps a slice of domain cost history records.
func NewSKUCostHistoryResponses(items []domainSKU.SKUCostHistory) []SKUCostHistoryResponse {
	out := make([]SKUCostHistoryResponse, len(items))
	for i := range items {
		out[i] = NewSKUCostHistoryResponse(&items[i])
	}
	return out
}
