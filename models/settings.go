package models

// CompanySettings represents company metadata
type CompanySettings struct {
	ID            uint   `gorm:"primaryKey" json:"-"`
	Name          string `json:"name"`
	TaxID         string `json:"taxId"`
	Address       string `json:"address"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Website       string `json:"website"`
	Currency      string `json:"currency"`
	VatRate       int    `json:"vatRate"`
	InvoicePrefix string `json:"invoicePrefix"`
	SoPrefix      string `json:"soPrefix"`
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
	ID         uint `gorm:"primaryKey" json:"-"`
	HourlyRate int  `json:"hourlyRate"`
	ClipBonus  int  `json:"clipBonus"`
}
