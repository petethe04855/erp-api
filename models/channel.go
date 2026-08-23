package models

import "time"

// TiktokConnection holds the server-side OAuth credentials for the TikTok Shop
// connection. Token values are encrypted before being persisted.
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

// TiktokOrder represents a TikTok shop sales order
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

// TiktokOrderItem stores each SKU returned by TikTok Shop for an order.
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

// ManualOrder represents sales orders recorded manually
type ManualOrder struct {
	ID       string  `gorm:"primaryKey" json:"id"`
	Customer string  `json:"customer"`
	Phone    string  `json:"phone"`
	Channel  string  `json:"channel"`
	Date     string  `json:"date"`
	Amount   float64 `json:"amount"`
	Status   string  `json:"status"`
	Items    int     `json:"items"`
	Notes    string  `json:"notes"`
}

// ContentScheduleItem represents live schedule slots
type ContentScheduleItem struct {
	ID        string `gorm:"primaryKey" json:"id"`
	Platform  string `json:"platform"`
	Account   string `json:"account"`
	Status    string `json:"status"` // scheduled, draft, done
	Topic     string `json:"topic"`
	Date      string `json:"date"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	CreatedAt string `json:"createdAt"`
}

// LiveSession represents staff payroll details for live events
type LiveSession struct {
	ID            string `gorm:"primaryKey" json:"id"`
	Platform      string `json:"platform"`
	Account       string `json:"account"`
	StaffName     string `json:"staffName"`
	Topic         string `json:"topic"`
	Date          string `json:"date"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	ClipsCount    int    `json:"clipsCount"`
	HoursCount    int    `json:"hoursCount"`
	HourlyRate    int    `json:"hourlyRate"`
	ClipBonus     int    `json:"clipBonus"`
	BaseSalary    int    `json:"baseSalary"`
	BonusPayout   int    `json:"bonusPayout"`
	TotalPayout   int    `json:"totalPayout"`
	Status        string `json:"status"` // draft, pending, approved
	ApprovedBy    string `json:"approvedBy"`
	CreatedBy     string `json:"createdBy"`
	UpdatedBy     string `json:"updatedBy"`
	UpdatedAt     string `json:"updatedAt"`
	AuditTrailStr string `json:"auditTrail"` // JSON string for ease of storing/displaying audit logs
}
