package models

// Product represents a master SKU product
type Product struct {
	ID          uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	SKU         string  `gorm:"uniqueIndex;not null" json:"sku"`
	Name        string  `json:"name"`
	Type        string  `json:"type"` // Raw Material, Packaging, Sub-component, Finished Product, Bundle, Cat, Dog, Other
	BaseUnit    string  `gorm:"default:'piece'" json:"baseUnit"`
	Stock       int     `gorm:"default:0" json:"stock"`
	ReservedQty int     `gorm:"default:0" json:"reservedQty"`
	Cost        float64 `gorm:"default:0" json:"cost"`
	IsBundle    bool    `json:"isBundle"`
	IsActive    bool    `gorm:"default:true" json:"isActive"`
	Note        string  `json:"note"`
	BomID       *uint   `gorm:"index" json:"bomId,omitempty"`
}

// BOM represents a standalone bill of materials / recipe (not tied to a product SKU)
type BOM struct {
	ID             uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code           string  `gorm:"uniqueIndex;not null" json:"code"` // e.g. BOM-001
	Name           string  `json:"name"`
	Version        int     `gorm:"default:1" json:"version"`
	Status         string  `gorm:"default:'Active'" json:"status"` // Draft, Active, Inactive
	Kind           string  `gorm:"default:'finished'" json:"kind"` // finished, subcomponent
	Waste          float64 `gorm:"default:0" json:"waste"`         // Waste %
	FgProductID    *uint   `gorm:"index" json:"fgProductId,omitempty"`
	FgSku          string  `json:"fgSku"`
	OutputQty      float64 `json:"outputQty"`
	OutputUnit     string  `json:"outputUnit"`
	EffectiveDate  string  `json:"effectiveDate"`
	Cost           float64 `json:"cost"`
	ComponentCount int     `json:"componentCount"`
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
	ID                 uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	BundleProductID    uint    `gorm:"index" json:"bundleProductId"`
	BundleSku          string  `gorm:"index" json:"bundleSku"`
	ComponentProductID uint    `gorm:"index" json:"componentProductId"`
	ComponentSku       string  `json:"componentSku"`
	ComponentName      string  `json:"componentName"`
	Qty                float64 `json:"qty"`
	Unit               string  `gorm:"not null;default:'piece'" json:"unit"`
	ScrapRate          float64 `gorm:"default:0" json:"scrapRate"`
	BOMLevel           int     `gorm:"default:1" json:"bomLevel"`
	Description        string  `json:"description"`
	ProcurementMethod  string  `gorm:"default:'Buy'" json:"procurementMethod"`
	Note               string  `json:"note"`
	ComponentType      string  `gorm:"not null;default:'material'" json:"componentType"`
	UnitCostOverride   float64 `json:"unitCostOverride"`
	YieldFactor        float64 `gorm:"default:1" json:"yieldFactor"` // 1 = ดิบ 1 ได้แห้ง 1, 0.25 = ดิบ 4 ได้แห้ง 1
}
