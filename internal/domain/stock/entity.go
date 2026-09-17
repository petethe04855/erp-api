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

type StockLot struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	SKUID        uint      `json:"sku_id" gorm:"column:sku_id;index;not null"`
	SKUCode      string    `json:"sku_code" gorm:"size:100"`
	WarehouseID  uint      `json:"warehouse_id" gorm:"index;default:1"`
	LotNumber    string    `json:"lot_number" gorm:"size:100;index"`
	SupplierLot  string    `json:"supplier_lot" gorm:"size:100"`
	ExpiryDate   string    `json:"expiry_date" gorm:"size:50;index"`
	Quantity     int       `json:"quantity" gorm:"default:0"`
	ReservedQty  int       `json:"reserved_qty" gorm:"default:0"`
	AvailableQty int       `json:"available_qty" gorm:"default:0"`
	ReceivedAt   time.Time `json:"received_at"`
	CreatedAt    time.Time `json:"created_at"`
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
	ID                uint         `json:"id" gorm:"primaryKey"`
	SKUID             uint         `json:"sku_id" gorm:"column:sku_id;index;not null"`
	SKUCode           string       `json:"sku_code" gorm:"size:100"`
	WarehouseID       uint         `json:"warehouse_id" gorm:"default:1"`
	StockLotID        *uint        `json:"stock_lot_id" gorm:"column:stock_lot_id;index"`
	SourceFormulaCode string       `json:"source_formula_code" gorm:"size:100"`
	Channel           string       `json:"channel" gorm:"size:50"`
	Type              MovementType `json:"type" gorm:"size:20;not null"`
	Quantity          int          `json:"quantity" gorm:"not null"`
	BeforeQty         int          `json:"before_qty"`
	AfterQty          int          `json:"after_qty"`
	ReferenceType     string       `json:"reference_type" gorm:"size:50"`
	ReferenceID       string       `json:"reference_id" gorm:"size:100"`
	Note              string       `json:"note" gorm:"size:255"`
	LotNumber         string       `json:"lot_number,omitempty" gorm:"-"`
	SupplierLot       string       `json:"supplier_lot,omitempty" gorm:"-"`
	ExpiryDate        string       `json:"expiry_date,omitempty" gorm:"-"`
	CreatedAt         time.Time    `json:"created_at"`
}

type Query struct {
	SKUID       uint
	WarehouseID uint
	Page        int
	Limit       int
}

type StockBySKU struct {
	SKUID          uint   `json:"sku_id"`
	SKUCode        string `json:"sku_code"`
	Quantity       int    `json:"quantity"`
	ReservedQty    int    `json:"reserved_qty"`
	AvailableQty   int    `json:"available_qty"`
	WarehouseCount int    `json:"warehouse_count"`
}

type StockBySKUQuery struct {
	Search string
	Page   int
	Limit  int
}

type Repository interface {
	GetBySKUID(ctx context.Context, skuID, warehouseID uint) (*Stock, error)
	// GetBySKUIDForUpdate locks the stock row (SELECT ... FOR UPDATE) so
	// availability checks and quantity updates share the same lock.
	GetBySKUIDForUpdate(ctx context.Context, skuID, warehouseID uint) (*Stock, error)
	FindAll(ctx context.Context, query Query) ([]Stock, int64, error)
	FindAllBySKU(ctx context.Context, query StockBySKUQuery) ([]StockBySKU, int64, error)
	UpdateQuantity(ctx context.Context, skuID, warehouseID uint, delta int) (*Stock, error)
	ReserveStock(ctx context.Context, skuID, warehouseID uint, qty int) (*Stock, error)
	ReleaseStock(ctx context.Context, skuID, warehouseID uint, qty int) (*Stock, error)
	CreateMovement(ctx context.Context, movement *StockMovement) error
	GetMovements(ctx context.Context, skuID uint, page, limit int) ([]StockMovement, int64, error)

	// Stock Lot & FEFO methods
	CreateLot(ctx context.Context, lot *StockLot) error
	GetAvailableLotsForUpdate(ctx context.Context, skuID, warehouseID uint) ([]StockLot, error)
	DeductLotQuantity(ctx context.Context, lotID uint, qty int) (*StockLot, error)
	FindLotsBySKU(ctx context.Context, skuID, warehouseID uint) ([]StockLot, error)
}

