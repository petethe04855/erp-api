// Package booth holds the booth point of sale: the booth's product buttons and
// set prices (one config row), and the bills rung up at the booth. Every bill
// takes its quantities out of the same stock the inventory page shows, inside
// one transaction, and a voided bill puts them back. Bills and movements are
// never deleted.
package booth

import (
	"context"
	"time"
)

// Limits and fixed values.
const (
	DefaultWarehouse = 1
	MaxLines         = 30
	MaxQtyPerLine    = 500
	MaxClientRef     = 64
	// Channel and reference types stamped on stock movements.
	Channel       = "BOOTH"
	RefTypeSale   = "BOOTH_SALE"
	RefTypeVoid   = "BOOTH_VOID"
	KindFlavor    = "flavor"
	KindItem      = "item"
	StatusDone    = "completed"
	StatusVoided  = "voided"
	PayCash       = "cash"
	PayTransfer   = "transfer"
	PayCard       = "card"
	DiscountBaht  = "baht"
	DiscountPct   = "percent"
	ConfigRowID   = 1
	MaxConfigRows = 30
)

// Flavor is a treat SKU sold by the bag. Bags of every flavor are priced
// together with the set prices, so a flavor has no price of its own.
type Flavor struct {
	SKUID uint   `json:"skuId"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// SetPrice is the price of a set of Size bags, any mix of flavors.
type SetPrice struct {
	Size  int     `json:"size"`
	Price float64 `json:"price"`
}

// Item is an SKU sold per piece at its own price, outside the sets.
type Item struct {
	SKUID uint    `json:"skuId"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// Discount is taken off a bill whose subtotal reaches Min.
type Discount struct {
	Enabled bool    `json:"enabled"`
	Min     float64 `json:"min"`
	Type    string  `json:"type"`
	Value   float64 `json:"value"`
}

// Config is what the booth sells and how it is priced.
type Config struct {
	WarehouseID uint       `json:"warehouseId"`
	Flavors     []Flavor   `json:"flavors"`
	Sets        []SetPrice `json:"sets"`
	Items       []Item     `json:"items"`
	Discount    Discount   `json:"discount"`
}

// ConfigRecord stores the config as one row (table booth_configs).
type ConfigRecord struct {
	ID        uint   `gorm:"primaryKey"`
	Data      Config `gorm:"serializer:json;type:jsonb"`
	UpdatedBy string `gorm:"size:255"`
	UpdatedAt time.Time
}

// TableName groups the booth tables.
func (ConfigRecord) TableName() string { return "booth_configs" }

// LotAlloc is how much of a line was taken from one stock lot (FEFO), so a
// void can put it back into the same lot.
type LotAlloc struct {
	LotID     uint   `json:"lotId"`
	LotNumber string `json:"lotNumber"`
	Qty       int    `json:"qty"`
}

// SaleLine is one SKU on a bill.
type SaleLine struct {
	SKUID     uint       `json:"skuId"`
	SKUCode   string     `json:"skuCode"`
	Name      string     `json:"name"`
	Kind      string     `json:"kind"`
	Qty       int        `json:"qty"`
	UnitPrice float64    `json:"unitPrice"`
	Amount    float64    `json:"amount"`
	Lots      []LotAlloc `json:"lots,omitempty"`
}

// TreatPart is how many times one set size was used to price the bags.
type TreatPart struct {
	Size      int     `json:"size"`
	Count     int     `json:"count"`
	UnitPrice float64 `json:"unitPrice"`
}

// Sale is one bill rung up at the booth (table booth_sales).
type Sale struct {
	ID            uint        `gorm:"primaryKey"`
	SaleNo        string      `gorm:"size:40;uniqueIndex"`
	ClientRef     string      `gorm:"size:64;uniqueIndex;not null"`
	WarehouseID   uint        `gorm:"not null"`
	SellerID      uint        `gorm:"index"`
	SellerName    string      `gorm:"size:255"`
	Lines         []SaleLine  `gorm:"serializer:json;type:jsonb"`
	Bags          int         `gorm:"not null;default:0"`
	TreatLabel    string      `gorm:"size:255"`
	TreatParts    []TreatPart `gorm:"serializer:json;type:jsonb"`
	TreatTotal    float64     `gorm:"type:numeric(12,2);default:0"`
	ItemsTotal    float64     `gorm:"type:numeric(12,2);default:0"`
	Subtotal      float64     `gorm:"type:numeric(12,2);default:0"`
	Discount      float64     `gorm:"type:numeric(12,2);default:0"`
	Total         float64     `gorm:"type:numeric(12,2);default:0"`
	PaymentMethod string      `gorm:"size:20;not null"`
	CashReceived  float64     `gorm:"type:numeric(12,2);default:0"`
	Change        float64     `gorm:"type:numeric(12,2);default:0"`
	Status        string      `gorm:"size:20;index;not null"`
	VoidedAt      *time.Time
	VoidedBy      string    `gorm:"size:255"`
	VoidReason    string    `gorm:"size:255"`
	CreatedAt     time.Time `gorm:"index"`
}

// TableName groups the booth tables.
func (Sale) TableName() string { return "booth_sales" }

// SaleFilter selects bills by the time they were rung up.
type SaleFilter struct {
	From  time.Time
	To    time.Time
	Limit int
}

// Repository stores the booth config and bills.
type Repository interface {
	GetConfig(ctx context.Context) (*ConfigRecord, error)
	SaveConfig(ctx context.Context, rec *ConfigRecord) error
	FindByClientRef(ctx context.Context, ref string) (*Sale, error)
	FindByIDForUpdate(ctx context.Context, id uint) (*Sale, error)
	Create(ctx context.Context, s *Sale) error
	SetSaleNo(ctx context.Context, id uint, no string) error
	MarkVoided(ctx context.Context, id uint, by, reason string, at time.Time) error
	List(ctx context.Context, f SaleFilter) ([]Sale, error)
}
