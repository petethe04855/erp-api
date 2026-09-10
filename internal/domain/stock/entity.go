package stock

import (
	"context"
	"time"
)

type Stock struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	SKUID        uint      `json:"sku_id" gorm:"column:sku_id;uniqueIndex:idx_sku_wh;not null"`
	SKUCode      string    `json:"sku_code" gorm:"size:100"`
	WarehouseID  uint      `json:"warehouse_id" gorm:"uniqueIndex:idx_sku_wh;default:1"`
	Quantity     int       `json:"quantity" gorm:"default:0"`
	ReservedQty  int       `json:"reserved_qty" gorm:"default:0"`
	AvailableQty int       `json:"available_qty" gorm:"default:0"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type MovementType string

const (
	MovementIn      MovementType = "IN"
	MovementOut     MovementType = "OUT"
	MovementAdjust  MovementType = "ADJUST"
	MovementReserve MovementType = "RESERVE"
	MovementRelease MovementType = "RELEASE"
)

type StockMovement struct {
	ID            uint         `json:"id" gorm:"primaryKey"`
	SKUID         uint         `json:"sku_id" gorm:"column:sku_id;index;not null"`
	SKUCode       string       `json:"sku_code" gorm:"size:100"`
	WarehouseID   uint         `json:"warehouse_id" gorm:"default:1"`
	Type          MovementType `json:"type" gorm:"size:20;not null"`
	Quantity      int          `json:"quantity" gorm:"not null"`
	BeforeQty     int          `json:"before_qty"`
	AfterQty      int          `json:"after_qty"`
	ReferenceType string       `json:"reference_type" gorm:"size:50"`
	ReferenceID   string       `json:"reference_id" gorm:"size:100"`
	Note          string       `json:"note" gorm:"size:255"`
	CreatedAt     time.Time    `json:"created_at"`
}

type Query struct {
	SKUID       uint
	WarehouseID uint
	Page        int
	Limit       int
}

type Repository interface {
	GetBySKUID(ctx context.Context, skuID, warehouseID uint) (*Stock, error)
	// GetBySKUIDForUpdate locks the stock row (SELECT ... FOR UPDATE) so
	// availability checks and quantity updates share the same lock.
	GetBySKUIDForUpdate(ctx context.Context, skuID, warehouseID uint) (*Stock, error)
	FindAll(ctx context.Context, query Query) ([]Stock, int64, error)
	UpdateQuantity(ctx context.Context, skuID, warehouseID uint, delta int) (*Stock, error)
	CreateMovement(ctx context.Context, movement *StockMovement) error
	GetMovements(ctx context.Context, skuID uint, page, limit int) ([]StockMovement, int64, error)
}
