package bundle

import (
	"context"
	"time"
)

type BundleItem struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	BundleSKU    string    `json:"bundle_sku" gorm:"index;not null;size:100"`
	ComponentSKU string    `json:"component_sku" gorm:"index;not null;size:100"`
	Quantity     int       `json:"quantity" gorm:"not null;default:1"`
	Note         string    `json:"note" gorm:"size:255"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ExplodedItem struct {
	ComponentSKU string `json:"component_sku"`
	Quantity     int    `json:"quantity"`
	AvailableQty int    `json:"available_qty"`
	HasStock     bool   `json:"has_stock"`
}

type Repository interface {
	GetItemsByBundleSKU(ctx context.Context, bundleSKU string) ([]BundleItem, error)
	// ListItems returns every stored bundle component row. Used by the
	// workspace bundle-components screen when no specific bundle is selected.
	ListItems(ctx context.Context) ([]BundleItem, error)
	SaveItems(ctx context.Context, bundleSKU string, items []BundleItem) error
	DeleteItem(ctx context.Context, id uint) error
}
