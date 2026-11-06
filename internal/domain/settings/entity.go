package settings

import (
	"context"
)

// CompanySettings represents company metadata and configurations
type CompanySettings struct {
	ID                  uint    `gorm:"primaryKey" json:"-"`
	Name                string  `json:"name" gorm:"size:255"`
	TaxID               string  `json:"taxId" gorm:"size:50"`
	Address             string  `json:"address" gorm:"type:text"`
	Phone               string  `json:"phone" gorm:"size:50"`
	Email               string  `json:"email" gorm:"size:255"`
	Website             string  `json:"website" gorm:"size:255"`
	Currency            string  `json:"currency" gorm:"size:10"`
	VatRate             float64 `json:"vatRate"`
	InvoicePrefix       string  `json:"invoicePrefix" gorm:"size:50"`
	SoPrefix            string  `json:"soPrefix" gorm:"size:50"`
	LogoURL             string  `json:"logoUrl" gorm:"size:500"`
	DefaultReorderPoint int     `json:"defaultReorderPoint" gorm:"default:10"`
}

// NotificationSettings represents warning thresholds
type NotificationSettings struct {
	ID             uint `gorm:"primaryKey" json:"-"`
	NearExpiry     bool `json:"nearExpiry"`
	NearExpiryDays int  `json:"nearExpiryDays"`
	LowStock       bool `json:"lowStock"`
	LatePO         bool `json:"latePO"`
	NewSO          bool `json:"newSO"`
	PaymentDue     bool `json:"paymentDue"`
}

// ModuleSettings represents which sidebar items are active
type ModuleSettings struct {
	ID               uint `gorm:"primaryKey" json:"-"`
	Quotation        bool `json:"quotation"`
	SalesOrders      bool `json:"salesOrders"`
	Invoice          bool `json:"invoice"`
	Returns          bool `json:"returns"`
	PurchaseReq      bool `json:"purchaseReq"`
	PurchaseOrder    bool `json:"purchaseOrder"`
	SkuMaster        bool `json:"skuMaster"`
	StockBalance     bool `json:"stockBalance"`
	GoodsReceive     bool `json:"goodsReceive"`
	GoodsIssue       bool `json:"goodsIssue"`
	StockTransfer    bool `json:"stockTransfer"`
	StockCheck       bool `json:"stockCheck"`
	Expenses         bool `json:"expenses"`
	PlReport         bool `json:"plReport"`
	Budget           bool `json:"budget"`
	TiktokOrders     bool `json:"tiktokOrders"`
	LiveContent      bool `json:"liveContent"`
	ManualOrder      bool `json:"manualOrder"`
	TiktokCalculator bool `json:"tiktokCalculator"`
	Sampling         bool `json:"sampling"`
	UserManagement   bool `json:"userManagement"`
	TiktokSetup      bool `json:"tiktokSetup"`
}

// LivePayrollSettings represents live staff wage rates
type LivePayrollSettings struct {
	ID         uint           `gorm:"primaryKey" json:"-"`
	HourlyRate int            `json:"hourlyRate"`
	ClipBonus  int            `json:"clipBonus"`
	StaffRates map[string]int `gorm:"serializer:json" json:"staffRates"`
}

// Settings aggregates all settings domains
type Settings struct {
	Company       CompanySettings      `json:"company"`
	Notifications NotificationSettings `json:"notifications"`
	Modules       ModuleSettings       `json:"modules"`
	LivePayroll   LivePayrollSettings  `json:"livePayroll"`
}

// Repository defines data access methods for settings
type Repository interface {
	GetSettings(ctx context.Context) (*Settings, error)
	UpdateCompany(ctx context.Context, comp *CompanySettings) (*CompanySettings, error)
	UpdateSettings(ctx context.Context, settings *Settings) (*Settings, error)
}
