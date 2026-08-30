package models

// Quotation represents a sales quotation
type Quotation struct {
	ID         uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Code       string          `gorm:"uniqueIndex;not null" json:"code"`
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
	ID          uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	QuotationID uint    `gorm:"index" json:"quotationId"`
	ProductID   uint    `gorm:"index" json:"productId"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	Qty         int     `json:"qty"`
	Price       float64 `json:"price"`
}

// SalesOrder represents a customer purchase
type SalesOrder struct {
	ID          uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string           `gorm:"uniqueIndex;not null" json:"code"`
	Customer    string           `json:"customer"`
	Date        string           `json:"date"`
	Amount      float64          `json:"amount"`
	TotalCOGS   float64          `gorm:"default:0" json:"totalCogs"`
	Status      string           `json:"status"`
	Channel     string           `json:"channel"`
	Items       int              `json:"items"`
	Lines       []SalesOrderLine `gorm:"foreignKey:SalesOrderID" json:"lines"`
	QuotationID *uint            `gorm:"index" json:"quotationId,omitempty"`
	QtRef       string           `json:"qtRef"`
	InvRef      string           `json:"invRef"`
	SourceRef   string           `json:"sourceRef"`
	AuditTrail  []AuditEvent     `gorm:"polymorphic:Owner;" json:"auditTrail"`
}

// SalesOrderLine represents items inside an order
type SalesOrderLine struct {
	ID           uint                   `gorm:"primaryKey;autoIncrement" json:"id"`
	SalesOrderID uint                   `gorm:"index" json:"salesOrderId"`
	ProductID    uint                   `gorm:"index" json:"productId"`
	SKU          string                 `gorm:"index" json:"sku"`
	Qty          int                    `json:"qty"`
	UnitPrice    float64                `gorm:"default:0" json:"unitPrice"`
	LineTotal    float64                `gorm:"default:0" json:"lineTotal"`
	UnitCost     float64                `gorm:"default:0" json:"unitCost"`
	TotalCost    float64                `gorm:"default:0" json:"totalCost"`
	Allocations  []SalesStockAllocation `gorm:"foreignKey:SalesOrderLineID" json:"allocations"`
}

// SalesStockAllocation records the exact FEFO lots and costs consumed by a sales line.
type SalesStockAllocation struct {
	ID               uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	SalesOrderID     uint    `gorm:"index;not null" json:"salesOrderId"`
	SalesOrderLineID uint    `gorm:"uniqueIndex:idx_sales_line_lot;not null" json:"salesOrderLineId"`
	StockLotID       uint    `gorm:"uniqueIndex:idx_sales_line_lot;not null" json:"stockLotId"`
	SKU              string  `gorm:"index" json:"sku"`
	Lot              string  `json:"lot"`
	Qty              int     `json:"qty"`
	UnitCost         float64 `json:"unitCost"`
	TotalCost        float64 `json:"totalCost"`
	ExpiryDate       string  `json:"expiryDate"`
}

// AuditEvent represents audit logs for entities like SO or Invoice
type AuditEvent struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	OwnerID   uint   `gorm:"index" json:"ownerId"`
	OwnerType string `gorm:"index" json:"ownerType"` // "sales_orders", "invoices", "purchase_orders", "goods_receives"
	Action    string `json:"action"`
	By        string `json:"by"`
	At        string `json:"at"`
	Note      string `json:"note"`
	Role      string `json:"role"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Reason    string `json:"reason"`
	SourceRef string `json:"sourceRef"`
}

// AuditLog is the central append-only audit record for cross-module actions.
type AuditLog struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Actor     string `gorm:"index;not null" json:"actor"`
	Role      string `gorm:"index" json:"role"`
	Action    string `gorm:"index;not null" json:"action"`
	Entity    string `gorm:"index;not null" json:"entity"`
	EntityID  string `gorm:"index" json:"entityId"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Reason    string `json:"reason"`
	SourceRef string `json:"sourceRef"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	CreatedAt string `gorm:"index;not null" json:"createdAt"`
}

// Invoice represents a billing invoice
type Invoice struct {
	ID               uint          `gorm:"primaryKey;autoIncrement" json:"id"`
	Code             string        `gorm:"uniqueIndex;not null" json:"code"`
	SalesOrderID     *uint         `gorm:"index" json:"salesOrderId,omitempty"`
	SoRef            string        `json:"soRef"`
	Customer         string        `json:"customer"`
	CustomerAddress  string        `json:"customerAddress"`
	CustomerTaxID    string        `json:"customerTaxId"`
	CustomerBranch   string        `json:"customerBranch"`
	PurchaseOrderRef string        `json:"purchaseOrderRef"`
	PaymentTerms     string        `json:"paymentTerms"`
	IssueDate        string        `json:"issueDate"`
	DueDate          string        `json:"dueDate"`
	Subtotal         float64       `gorm:"default:0" json:"subtotal"`
	VATAmount        float64       `gorm:"default:0" json:"vatAmount"`
	Amount           float64       `json:"amount"`
	Paid             float64       `json:"paid"`
	Credited         float64       `gorm:"default:0" json:"credited"`
	RefundDue        float64       `gorm:"default:0" json:"refundDue"`
	Status           string        `json:"status"`
	Lines            []InvoiceLine `gorm:"foreignKey:InvoiceID" json:"lines"`
	AuditTrail       []AuditEvent  `gorm:"polymorphic:Owner;" json:"auditTrail"`
}

// InvoiceLine is an immutable snapshot of the product and price billed to the customer.
type InvoiceLine struct {
	ID        uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	InvoiceID uint    `gorm:"index;not null" json:"invoiceId"`
	ProductID *uint   `gorm:"index" json:"productId,omitempty"`
	SKU       string  `json:"sku"`
	Lot       string  `json:"lot"`
	Name      string  `json:"name"`
	Qty       int     `json:"qty"`
	Unit      string  `json:"unit"`
	UnitPrice float64 `json:"unitPrice"`
	Discount  float64 `gorm:"default:0" json:"discount"`
	LineTotal float64 `json:"lineTotal"`
}
