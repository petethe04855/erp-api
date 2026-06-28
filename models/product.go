package models

// Product represents a master SKU product
type Product struct {
	SKU            string  `gorm:"primaryKey" json:"sku"`
	Name           string  `json:"name"`
	Type           string  `json:"type"` // Cat, Dog, Bundle, Other
	Barcode        string  `json:"barcode"`
	WeightGrams    int     `json:"weightGrams"`
	RetailPrice    float64 `json:"retailPrice"`
	WholesalePrice float64 `json:"wholesalePrice"`
	Price          float64 `json:"price"` // Legacy alias
	Cost           float64 `json:"cost"`
	Stock          int     `json:"stock"`
	Reorder        int     `json:"reorder"`
	ReservedQty    int     `json:"reservedQty"`
	IsBundle         bool    `json:"isBundle"`
	IsActive         bool    `json:"isActive"`
	Note             string  `json:"note"`
	BaseUnit         string  `gorm:"not null;default:'piece'" json:"baseUnit"`
}

// BOM represents a standalone bill of materials / recipe (not tied to a product SKU)
type BOM struct {
	ID               uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code             string  `gorm:"uniqueIndex;not null" json:"code"` // e.g. BOM-001
	Name             string  `json:"name"`
	OutputQty        float64 `json:"outputQty"`
	OutputUnit       string  `json:"outputUnit"`
	Status           string  `json:"status"` // Draft, Active, Inactive
	EffectiveDate    string  `json:"effectiveDate"`
	Cost             float64 `json:"cost"`
	ComponentCount   int     `json:"componentCount"`
	// Legacy fields kept for backward compat with product-linked BOM handlers
	BomCode          string  `json:"-" gorm:"-"`
	BomName          string  `json:"-" gorm:"-"`
	BomOutputQty     float64 `json:"-" gorm:"-"`
	BomUnit          string  `json:"-" gorm:"-"`
	BomStatus        string  `json:"-" gorm:"-"`
	BomEffectiveDate string  `json:"-" gorm:"-"`
}

// BundleComponent maps bundle products to component SKUs
type BundleComponent struct {
	ID               uint    `gorm:"primaryKey" json:"-"`
	BundleSku        string  `gorm:"index" json:"bundleSku"`
	ComponentSku     string  `json:"componentSku"`
	ComponentName    string  `json:"componentName"`
	Qty              float64 `json:"qty"`
	Unit             string  `gorm:"not null;default:'piece'" json:"unit"`
	ComponentType    string  `gorm:"not null;default:'material'" json:"componentType"`
	UnitCostOverride float64 `json:"unitCostOverride"`
	YieldFactor      float64 `gorm:"default:1" json:"yieldFactor"` // 1 = ดิบ 1 ได้แห้ง 1, 0.25 = ดิบ 4 ได้แห้ง 1
}
