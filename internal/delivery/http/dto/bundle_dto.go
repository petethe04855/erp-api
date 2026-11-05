package dto

import (
	"time"

	domainBundle "chawy-erp-api/internal/domain/bundle"
)

type SetBundleRequest struct {
	Components []BundleComponentDTO `json:"components"`
}

type BundleComponentDTO struct {
	ComponentSKU string `json:"component_sku"`
	Quantity     int    `json:"quantity"`
	Note         string `json:"note"`
}

type ExplodeResponse struct {
	BundleSKU  string                 `json:"bundle_sku"`
	OrderQty   int                    `json:"order_qty"`
	AllInStock bool                   `json:"all_in_stock"`
	Components []ExplodedComponentDTO `json:"components"`
}

type ExplodedComponentDTO struct {
	ComponentSKU string `json:"component_sku"`
	Quantity     int    `json:"quantity"`
	AvailableQty int    `json:"available_qty"`
	HasStock     bool   `json:"has_stock"`
}

// BundleItemResponse is the API contract for a stored bundle component. Field
// names match the previous domain-entity output exactly so existing clients
// (bundle editors, workspace screens) are unaffected.
type BundleItemResponse struct {
	ID           uint      `json:"id"`
	BundleSKU    string    `json:"bundle_sku"`
	ComponentSKU string    `json:"component_sku"`
	Quantity     int       `json:"quantity"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewBundleItemResponse maps a domain bundle item to the API contract.
func NewBundleItemResponse(item *domainBundle.BundleItem) BundleItemResponse {
	return BundleItemResponse{
		ID:           item.ID,
		BundleSKU:    item.BundleSKU,
		ComponentSKU: item.ComponentSKU,
		Quantity:     item.Quantity,
		Note:         item.Note,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

// NewBundleItemResponses maps a slice of domain bundle items.
func NewBundleItemResponses(items []domainBundle.BundleItem) []BundleItemResponse {
	out := make([]BundleItemResponse, len(items))
	for i := range items {
		out[i] = NewBundleItemResponse(&items[i])
	}
	return out
}
