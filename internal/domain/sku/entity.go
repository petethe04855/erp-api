package sku

import (
	"context"
	"time"
)

type SKU struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	SKU       string    `json:"sku" gorm:"uniqueIndex;not null;size:100"`
	Name      string    `json:"name" gorm:"not null;size:255"`
	Barcode   string    `json:"barcode" gorm:"size:100;index"`
	Category  string    `json:"category" gorm:"size:100"`
	Price     float64   `json:"price" gorm:"type:numeric(12,2);default:0"`
	CostPrice float64   `json:"cost_price" gorm:"type:numeric(12,2);default:0"`
	IsBundle  bool      `json:"is_bundle" gorm:"default:false"`
	Image        string    `json:"image" gorm:"size:500"`
	Status       string    `json:"status" gorm:"size:50;default:'active'"`
	ReorderPoint int       `json:"reorder_point" gorm:"default:10"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Query struct {
	Search   string
	Category string
	Status   string
	Page     int
	Limit    int
}

// SKUReceiptItem represents a unified receipt record for an SKU
type SKUReceiptItem struct {
	ID               uint      `json:"id"`
	ReceivedAt       time.Time `json:"received_at"`
	SourceType       string    `json:"source_type"` // INITIAL_STOCK, GOODS_RECEIVE, STOCK_ADJUSTMENT_IN
	Quantity         int       `json:"quantity"`
	UnitCost         float64   `json:"unit_cost"`
	RetailPrice      float64   `json:"retail_price"`
	WarehouseID      uint      `json:"warehouse_id"`
	WarehouseName    string    `json:"warehouse_name"`
	LotNumber        string    `json:"lot_number"`
	SupplierLot      string    `json:"supplier_lot"`
	ExpiryDate       string    `json:"expiry_date"`
	ReferenceType    string    `json:"reference_type"`
	ReferenceID      string    `json:"reference_id"`
	PurchaseOrderRef string    `json:"purchase_order_ref"`
	Note             string    `json:"note"`
}

type SKUReceiptQuery struct {
	SKUID       uint
	WarehouseID uint
	Page        int
	Limit       int
}

type SKUBatchReceiptStat struct {
	SKUID          uint
	LastReceivedAt *time.Time
	ReceiptCount   int
}

type Repository interface {
	Create(ctx context.Context, sku *SKU) error
	FindByID(ctx context.Context, id uint) (*SKU, error)
	FindBySKU(ctx context.Context, skuCode string) (*SKU, error)
	FindAll(ctx context.Context, query Query) ([]SKU, int64, error)
	Update(ctx context.Context, sku *SKU) error
	Delete(ctx context.Context, id uint) error
	ExistsBySKU(ctx context.Context, skuCode string) (bool, error)

	// Receipt stats & history
	GetReceiptStatsBatch(ctx context.Context, skuIDs []uint) (map[uint]SKUBatchReceiptStat, error)
	GetReceiptHistory(ctx context.Context, query SKUReceiptQuery) ([]SKUReceiptItem, int64, error)
}

