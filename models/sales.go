package models

// Quotation represents a sales quotation
type Quotation struct {
	ID         string          `gorm:"primaryKey" json:"id"`
	Customer   string          `json:"customer"`
	Date       string          `json:"date"`
	ValidUntil string          `json:"validUntil"`
	LeadSource string          `json:"leadSource"`
	Amount     float64         `json:"amount"`
	Status     string          `json:"status"`
	Lines      []QuotationLine `gorm:"foreignKey:QuotationID" json:"lines"`
}

// QuotationLine represents items inside a quotation
type QuotationLine struct {
	ID          uint    `gorm:"primaryKey" json:"-"`
	QuotationID string  `gorm:"index" json:"-"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	Qty         int     `json:"qty"`
	Price       float64 `json:"price"`
}

// SalesOrder represents a customer purchase
type SalesOrder struct {
	ID         string           `gorm:"primaryKey" json:"id"`
	Customer   string           `json:"customer"`
	Date       string           `json:"date"`
	Amount     float64          `json:"amount"`
	Status     string           `json:"status"`
	Channel    string           `json:"channel"`
	Items      int              `json:"items"`
	Lines      []SalesOrderLine `gorm:"foreignKey:OrderID" json:"lines"`
	QtRef      string           `json:"qtRef"`
	InvRef     string           `json:"invRef"`
	SourceRef  string           `json:"sourceRef"`
	AuditTrail []AuditEvent     `gorm:"polymorphic:Owner;" json:"auditTrail"`
}

// SalesOrderLine represents items inside an order
type SalesOrderLine struct {
	ID      uint   `gorm:"primaryKey" json:"-"`
	OrderID string `gorm:"index" json:"-"`
	SKU     string `json:"sku"`
	Qty     int    `json:"qty"`
}

// AuditEvent represents audit logs for entities like SO or Invoice
type AuditEvent struct {
	ID        uint   `gorm:"primaryKey" json:"-"`
	OwnerID   string `gorm:"index" json:"-"`
	OwnerType string `gorm:"index" json:"-"` // "sales_orders", "invoices", "purchase_orders", "goods_receives"
	Action    string `json:"action"`
	By        string `json:"by"`
	At        string `json:"at"`
	Note      string `json:"note"`
}

// Invoice represents a billing invoice
type Invoice struct {
	ID         string       `gorm:"primaryKey" json:"id"`
	SoRef      string       `json:"soRef"`
	Customer   string       `json:"customer"`
	IssueDate  string       `json:"issueDate"`
	DueDate    string       `json:"dueDate"`
	Amount     float64      `json:"amount"`
	Paid       float64      `json:"paid"`
	Status     string       `json:"status"`
	AuditTrail []AuditEvent `gorm:"polymorphic:Owner;" json:"auditTrail"`
}
