package sku

import (
	"context"
	"time"
)

// SKUAccessory represents a packaging or accessory item linked to a non-bundle SKU
// that should be deducted from inventory when the parent SKU is shipped.
type SKUAccessory struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	SKU          string    `json:"sku" gorm:"index;not null;size:100"`
	AccessorySKU string    `json:"accessory_sku" gorm:"index;not null;size:100"`
	Quantity     int       `json:"quantity" gorm:"not null;default:1"`
	Note         string    `json:"note" gorm:"size:255"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AccessoryRepository defines operations for managing SKU accessories.
type AccessoryRepository interface {
	GetBySKU(ctx context.Context, sku string) ([]SKUAccessory, error)
	SaveAccessories(ctx context.Context, sku string, items []SKUAccessory) error
	DeleteBySKU(ctx context.Context, sku string) error
	CountByAccessorySKU(ctx context.Context, accessorySKU string) (int64, error)
}
