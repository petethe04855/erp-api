package purchasing

import (
	"context"
	"time"
)

type POStatus string

const (
	StatusPending   POStatus = "PENDING"
	StatusApproved  POStatus = "APPROVED"
	StatusReceived  POStatus = "RECEIVED"
	StatusCancelled POStatus = "CANCELLED"
)

type Supplier struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Code          string    `json:"code" gorm:"uniqueIndex;size:50"`
	Name          string    `json:"name" gorm:"not null;size:255"`
	ContactPerson string    `json:"contact_person" gorm:"size:255"`
	Phone         string    `json:"phone" gorm:"size:50"`
	Email         string    `json:"email" gorm:"size:255"`
	Address       string    `json:"address" gorm:"type:text"`
	Status        string    `json:"status" gorm:"size:50;default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type PurchaseOrder struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	PONo         string    `json:"po_no" gorm:"uniqueIndex;not null;size:100"`
	SupplierID   uint      `json:"supplier_id" gorm:"index"`
	SupplierName string    `json:"supplier_name" gorm:"size:255"`
	Status       POStatus  `json:"status" gorm:"size:50;default:'PENDING'"`
	TotalAmount  float64   `json:"total_amount" gorm:"type:numeric(12,2);default:0"`
	Note         string    `json:"note" gorm:"size:255"`
	ETADate      string    `json:"eta_date" gorm:"size:50"`
	Items        []POItem  `json:"items" gorm:"foreignKey:POID"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type POItem struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	POID        uint      `json:"po_id" gorm:"index;not null"`
	SKU         string    `json:"sku" gorm:"size:100;not null"`
	Name        string    `json:"name" gorm:"size:255"`
	UnitCost    float64   `json:"unit_cost" gorm:"type:numeric(12,2);default:0"`
	Quantity    int       `json:"quantity" gorm:"not null;default:1"`
	ReceivedQty int       `json:"received_qty" gorm:"default:0"`
	Subtotal    float64   `json:"subtotal" gorm:"type:numeric(12,2);default:0"`
	CreatedAt   time.Time `json:"created_at"`
}

// GoodsReceive represents a persistent Goods Receipt document
type GoodsReceive struct {
	ID           uint               `json:"id" gorm:"primaryKey"`
	Code         string             `json:"code" gorm:"uniqueIndex;not null;size:100"`
	POID         *uint              `json:"po_id" gorm:"index"`
	PORef        string             `json:"po_ref" gorm:"size:100"`
	SupplierName string             `json:"supplier_name" gorm:"size:255"`
	ReceiveDate  string             `json:"receive_date" gorm:"size:50"`
	Note         string             `json:"note" gorm:"size:255"`
	Items        []GoodsReceiveItem `json:"items" gorm:"foreignKey:GRID"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

func (GoodsReceive) TableName() string { return "goods_receives" }

// GoodsReceiveItem represents items in a Goods Receipt
type GoodsReceiveItem struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	GRID        uint      `json:"gr_id" gorm:"index;not null"`
	SKU         string    `json:"sku" gorm:"size:100;not null"`
	Name        string    `json:"name" gorm:"size:255"`
	Quantity    int       `json:"qty" gorm:"not null"`
	SupplierLot string    `json:"supplier_lot" gorm:"size:100"`
	ExpiryDate  string    `json:"expiry_date" gorm:"size:50"`
	QCStatus    string    `json:"qc_status" gorm:"size:50"`
	CreatedAt   time.Time `json:"created_at"`
}

func (GoodsReceiveItem) TableName() string { return "goods_receive_items" }

type SupplierQuery struct {
	Search string
	Status string
	Page   int
	Limit  int
}

type POQuery struct {
	Search     string
	SupplierID uint
	Status     string
	Page       int
	Limit      int
}

type Repository interface {
	// Supplier
	CreateSupplier(ctx context.Context, supplier *Supplier) error
	FindSupplierByID(ctx context.Context, id uint) (*Supplier, error)
	FindAllSuppliers(ctx context.Context, query SupplierQuery) ([]Supplier, int64, error)
	UpdateSupplier(ctx context.Context, supplier *Supplier) error
	DeleteSupplier(ctx context.Context, id uint) error

	// Purchase Order
	CreatePO(ctx context.Context, po *PurchaseOrder) error
	FindPOByID(ctx context.Context, id uint) (*PurchaseOrder, error)
	// FindPOByIDForUpdate locks the PO row (SELECT ... FOR UPDATE) within the
	// caller's transaction so concurrent receives serialize (FULL-09).
	FindPOByIDForUpdate(ctx context.Context, id uint) (*PurchaseOrder, error)
	FindAllPOs(ctx context.Context, query POQuery) ([]PurchaseOrder, int64, error)
	UpdatePOStatus(ctx context.Context, id uint, status POStatus) error
	UpdatePO(ctx context.Context, po *PurchaseOrder) error
	UpdatePOItemReceivedQty(ctx context.Context, poItemID uint, receivedQty int) error

	// Goods Receive documents
	CreateGoodsReceiveDoc(ctx context.Context, gr *GoodsReceive, items []GoodsReceiveItem) error
}
