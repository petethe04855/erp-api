package tiktok

import (
	"context"
	"time"
)

// TiktokConnection holds OAuth tokens and store information
type TiktokConnection struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	AccessToken           string    `json:"-"`
	RefreshToken          string    `json:"-"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
	ShopCipher            string    `json:"shopCipher"`
	SellerName            string    `json:"sellerName"`
	SellerBaseRegion      string    `json:"sellerBaseRegion"`
	GrantedScopes         string    `json:"grantedScopes"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

func (TiktokConnection) TableName() string { return "tiktok_connections" }

// TiktokOAuthState stores CSRF state hashes during OAuth authorization
type TiktokOAuthState struct {
	ID        uint       `gorm:"primaryKey" json:"-"`
	StateHash string     `gorm:"uniqueIndex;size:64;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"-"`
	UsedAt    *time.Time `gorm:"index" json:"-"`
	CreatedAt time.Time  `json:"-"`
}

func (TiktokOAuthState) TableName() string { return "tiktok_oauth_states" }

// TiktokWebhookEvent records incoming webhook payloads for auditing and deduplication
type TiktokWebhookEvent struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID    string    `gorm:"uniqueIndex;not null" json:"eventId"`
	EventType  string    `json:"eventType"`
	Payload    string    `gorm:"type:text" json:"-"`
	ReceivedAt time.Time `json:"receivedAt"`
}

func (TiktokWebhookEvent) TableName() string { return "tiktok_webhook_events" }

// TiktokSyncRun tracks history of sync operations
type TiktokSyncRun struct {
	ID            uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
	Status        string     `json:"status"`
	Days          int        `json:"days"`
	Synced        int        `json:"synced"`
	StockDeducted int        `json:"stockDeducted"`
	Error         string     `json:"error,omitempty"`
}

func (TiktokSyncRun) TableName() string { return "tiktok_sync_runs" }

// TiktokOrder stores order headers received from TikTok Shop
type TiktokOrder struct {
	ID            string            `gorm:"primaryKey" json:"id"`
	Date          string            `json:"date"`
	Product       string            `json:"product"`
	SKU           string            `json:"sku"`
	Qty           int               `json:"qty"`
	Amount        float64           `json:"amount"`
	Status        string            `json:"status"`
	StockDeducted bool              `json:"stockDeducted"`
	Imported      bool              `json:"imported"`
	NetRevenue    float64           `json:"netRevenue,omitempty"`
	PlatformFee   float64           `json:"platformFee,omitempty"`
	Settled       bool              `json:"settled"`
	SettlementRef string            `json:"settlementRef,omitempty"`
	Items         []TiktokOrderItem `gorm:"foreignKey:OrderID;references:ID;constraint:OnDelete:CASCADE" json:"items"`
}

func (TiktokOrder) TableName() string { return "tiktok_orders" }

// TiktokOrderItem stores SKU lines for each TikTok order
type TiktokOrderItem struct {
	ID          uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	OrderID     string  `gorm:"index;not null" json:"orderId"`
	LineItemID  string  `gorm:"index" json:"lineItemId"`
	ProductName string  `json:"productName"`
	SKU         string  `json:"sku"`
	Qty         int     `json:"qty"`
	UnitPrice   float64 `json:"unitPrice"`
	Amount      float64 `json:"amount"`
}

func (TiktokOrderItem) TableName() string { return "tiktok_order_items" }

// SKUMapping maps TikTok Seller SKU to ERP Master SKU
type SKUMapping struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TikTokSKU string    `json:"tiktok_sku" gorm:"column:tiktok_sku;uniqueIndex;not null;size:100"`
	LocalSKU  string    `json:"local_sku" gorm:"column:erp_sku;index;not null;size:100"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SKUMapping) TableName() string { return "tiktok_sku_mappings" }

// SyncLog keeps simple informational logs for webhook/API calls
type SyncLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	OrderNo   string    `json:"order_no" gorm:"index;size:100"`
	Status    string    `json:"status" gorm:"size:50"`
	Message   string    `json:"message" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}

func (SyncLog) TableName() string { return "sync_logs" }

// Repository outlines database operations for TikTok integration
type Repository interface {
	// Connection operations
	GetConnection(ctx context.Context) (*TiktokConnection, error)
	SaveConnection(ctx context.Context, conn *TiktokConnection) error

	// OAuth State operations
	CreateOAuthState(ctx context.Context, state *TiktokOAuthState) error
	ValidateAndConsumeOAuthState(ctx context.Context, stateHash string) (bool, error)

	// Webhook operations
	IsWebhookEventRecorded(ctx context.Context, eventID string) (bool, error)
	RecordWebhookEvent(ctx context.Context, event *TiktokWebhookEvent) error

	// Orders & Line items
	UpsertOrders(ctx context.Context, orders []TiktokOrder) error
	GetOrderByID(ctx context.Context, id string) (*TiktokOrder, error)
	UpdateOrderStockDeducted(ctx context.Context, orderID string, deducted bool) error
	ListRecentOrders(ctx context.Context, limit int) ([]TiktokOrder, error)

	// Sync Runs
	CreateSyncRun(ctx context.Context, run *TiktokSyncRun) error
	UpdateSyncRun(ctx context.Context, run *TiktokSyncRun) error
	ListSyncRuns(ctx context.Context, limit int) ([]TiktokSyncRun, error)

	// SKU Mappings
	SaveMapping(ctx context.Context, m *SKUMapping) error
	GetMapping(ctx context.Context, tiktokSKU string) (*SKUMapping, error)
	ListMappings(ctx context.Context) ([]SKUMapping, error)

	// Sync Logs
	CreateSyncLog(ctx context.Context, log *SyncLog) error
	GetSyncLogs(ctx context.Context, limit int) ([]SyncLog, error)
}
