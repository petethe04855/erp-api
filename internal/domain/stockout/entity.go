// Package stockout holds the daily stock-out count: packers tap what they
// pick from stock, save it as one batch, and the server takes it out of the
// inventory lot by lot (FEFO). A short batch is still recorded (the goods are
// really in hand), takes what the lots hold and waits for the owner's review.
// Batches and movements are never deleted; a void puts stock back into the
// same lots.
package stockout

import (
	"context"
	"time"
)

// Fixed values.
const (
	DefaultWarehouse = 1
	MaxLines         = 40
	MaxQtyPerLine    = 1000
	MaxClientID      = 64
	MaxBoxes         = 40
	ConfigRowID      = 1
	// OwnVoidWindow is how long, after the server recorded a batch, the
	// person who counted it may void it without the void permission: the
	// screen shows its undo for 6 seconds, plus a few seconds for the request
	// to arrive.
	OwnVoidWindow = 10 * time.Second
	// ClientClockSkew is how far a device clock may run ahead.
	ClientClockSkew = 5 * time.Minute
	// MaxBackdate is how old an offline batch may be when it finally syncs.
	MaxBackdate = 7 * 24 * time.Hour

	Channel        = "STOCK_OUT"
	RefTypeOut     = "STOCK_OUT"
	RefTypeVoid    = "STOCK_OUT_VOID"
	StatusPosted   = "posted"
	StatusVoided   = "voided"
	StatusReview   = "needs_review"
	KindFlavor     = "flavor"
	KindItem       = "item"
	MaxHistoryDays = 92
)

// Box is one tile on the count screen, mapped to an ERP SKU.
type Box struct {
	SKUID    uint   `json:"skuId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`  // flavor = big colored tile, item = plain tile
	Group    string `json:"group"` // boxes sharing a group are also totalled together (e.g. wand heads)
	Unit     string `json:"unit"`
	Color    string `json:"color"`
	Image    string `json:"image"`
	LowStock int    `json:"lowStock"`
}

// Config is what the count screen shows and who may do what.
type Config struct {
	WarehouseID uint     `json:"warehouseId"`
	Boxes       []Box    `json:"boxes"`
	CountRoles  []string `json:"countRoles"`
	VoidRoles   []string `json:"voidRoles"`
}

// ConfigRecord stores the config as one row (table stockout_configs).
type ConfigRecord struct {
	ID        uint   `gorm:"primaryKey"`
	Data      Config `gorm:"serializer:json;type:jsonb"`
	UpdatedBy string `gorm:"size:255"`
	UpdatedAt time.Time
}

// TableName groups the stock-out tables.
func (ConfigRecord) TableName() string { return "stockout_configs" }

// LotTake is how much of a line came out of one lot.
type LotTake struct {
	LotID     uint   `json:"lotId"`
	LotNumber string `json:"lotNumber"`
	Expiry    string `json:"expiry"`
	Qty       int    `json:"qty"`
}

// Line is one SKU in a batch. Qty is what was counted; Deducted is what the
// lots held; Short is the rest, which the owner has to review.
type Line struct {
	SKUID    uint      `json:"skuId"`
	SKUCode  string    `json:"skuCode"`
	Name     string    `json:"name"`
	Group    string    `json:"group"`
	Qty      int       `json:"qty"`
	Deducted int       `json:"deducted"`
	Short    int       `json:"short"`
	Lots     []LotTake `json:"lots"`
}

// Batch is one saved count (table stockout_batches).
type Batch struct {
	ID          uint   `gorm:"primaryKey"`
	BatchNo     string `gorm:"size:40;uniqueIndex"`
	ClientID    string `gorm:"size:64;uniqueIndex;not null"`
	WarehouseID uint   `gorm:"not null"`
	UserID      uint   `gorm:"index"`
	UserName    string `gorm:"size:255"`
	Lines       []Line `gorm:"serializer:json;type:jsonb"`
	TotalQty    int    `gorm:"not null;default:0"`
	Status      string `gorm:"size:20;index;not null"`
	VoidedAt    *time.Time
	VoidedBy    string `gorm:"size:255"`
	ReviewedAt  *time.Time
	ReviewedBy  string    `gorm:"size:255"`
	ReviewNote  string    `gorm:"size:255"`
	CountedAt   time.Time `gorm:"index;not null"` // when it was tapped (device time, checked)
	CreatedAt   time.Time // when the server recorded it
}

// TableName groups the stock-out tables.
func (Batch) TableName() string { return "stockout_batches" }

// Filter selects batches by the time they were counted.
type Filter struct {
	From  time.Time
	To    time.Time
	Limit int
}

// Repository stores the config and batches.
type Repository interface {
	GetConfig(ctx context.Context) (*ConfigRecord, error)
	SaveConfig(ctx context.Context, rec *ConfigRecord) error
	FindByClientID(ctx context.Context, clientID string) (*Batch, error)
	FindByIDForUpdate(ctx context.Context, id uint) (*Batch, error)
	Create(ctx context.Context, b *Batch) error
	SetBatchNo(ctx context.Context, id uint, no string) error
	MarkVoided(ctx context.Context, id uint, by string, at time.Time) error
	MarkReviewed(ctx context.Context, id uint, by, note string, at time.Time) error
	List(ctx context.Context, f Filter) ([]Batch, error)
}
