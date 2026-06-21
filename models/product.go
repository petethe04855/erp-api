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
	IsBundle       bool    `json:"isBundle"`
	IsActive       bool    `json:"isActive"`
	Note           string  `json:"note"`
	BaseUnit       string  `gorm:"not null;default:'piece'" json:"baseUnit"`
}

// BundleComponent maps bundle products to component SKUs
type BundleComponent struct {
	ID               uint    `gorm:"primaryKey" json:"-"`
	BundleSku        string  `gorm:"index" json:"bundleSku"`
	ComponentSku     string  `json:"componentSku"`
	Qty              float64 `json:"qty"`
	Unit             string  `gorm:"not null;default:'piece'" json:"unit"`
	ComponentType    string  `gorm:"not null;default:'material'" json:"componentType"`
	UnitCostOverride float64 `json:"unitCostOverride"`
}
