package dto

type SetBundleRequest struct {
	Components []BundleComponentDTO `json:"components"`
}

type BundleComponentDTO struct {
	ComponentSKU string `json:"component_sku"`
	Quantity     int    `json:"quantity"`
	Note         string `json:"note"`
}

type ExplodeResponse struct {
	BundleSKU  string                   `json:"bundle_sku"`
	OrderQty   int                      `json:"order_qty"`
	AllInStock bool                     `json:"all_in_stock"`
	Components []ExplodedComponentDTO   `json:"components"`
}

type ExplodedComponentDTO struct {
	ComponentSKU string `json:"component_sku"`
	Quantity     int    `json:"quantity"`
	AvailableQty int    `json:"available_qty"`
	HasStock     bool   `json:"has_stock"`
}
