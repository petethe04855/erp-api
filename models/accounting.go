package models

// Account is a minimal chart-of-accounts record used by automatic posting.
type Account struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code     string `gorm:"uniqueIndex;not null" json:"code"`
	Name     string `json:"name"`
	Type     string `json:"type"` // Asset, Liability, Equity, Revenue, Expense
	IsActive bool   `gorm:"default:true" json:"isActive"`
}

// JournalEntry is an immutable posted accounting document.
type JournalEntry struct {
	ID           uint          `gorm:"primaryKey;autoIncrement" json:"id"`
	Code         string        `gorm:"uniqueIndex;not null" json:"code"`
	Date         string        `gorm:"index" json:"date"`
	SourceType   string        `gorm:"uniqueIndex:idx_journal_source;not null" json:"sourceType"`
	SourceID     uint          `gorm:"uniqueIndex:idx_journal_source;not null" json:"sourceId"`
	SourceRef    string        `gorm:"index" json:"sourceRef"`
	Description  string        `json:"description"`
	Status       string        `gorm:"default:'Posted'" json:"status"`
	ReversalOfID *uint         `gorm:"index" json:"reversalOfId,omitempty"`
	CreatedBy    string        `json:"createdBy"`
	PostedAt     string        `json:"postedAt"`
	Lines        []JournalLine `gorm:"foreignKey:JournalEntryID" json:"lines"`
}

// JournalLine is one debit or credit leg of a journal entry.
type JournalLine struct {
	ID             uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	JournalEntryID uint    `gorm:"index;not null" json:"journalEntryId"`
	AccountID      uint    `gorm:"index;not null" json:"accountId"`
	AccountCode    string  `gorm:"index" json:"accountCode"`
	AccountName    string  `json:"accountName"`
	Debit          float64 `gorm:"default:0" json:"debit"`
	Credit         float64 `gorm:"default:0" json:"credit"`
	SKU            string  `gorm:"index" json:"sku"`
	Lot            string  `json:"lot"`
	Channel        string  `gorm:"index" json:"channel"`
}

// CustomerPayment keeps each receipt separate so partial payments can post exactly once.
type CustomerPayment struct {
	ID          uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string  `gorm:"uniqueIndex;not null" json:"code"`
	InvoiceID   uint    `gorm:"index;not null" json:"invoiceId"`
	InvoiceRef  string  `gorm:"index" json:"invoiceRef"`
	Date        string  `json:"date"`
	Amount      float64 `json:"amount"`
	AccountCode string  `json:"accountCode"`
	Method      string  `json:"method"`
	Reference   string  `json:"reference"`
	CreatedBy   string  `json:"createdBy"`
}

// CreditNote reduces a customer's invoice balance without deleting the original invoice.
type CreditNote struct {
	ID            uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code          string  `gorm:"uniqueIndex;not null" json:"code"`
	StockReturnID uint    `gorm:"uniqueIndex;not null" json:"stockReturnId"`
	InvoiceID     *uint   `gorm:"index" json:"invoiceId,omitempty"`
	InvoiceRef    string  `gorm:"index" json:"invoiceRef"`
	SalesOrderID  uint    `gorm:"index;not null" json:"salesOrderId"`
	SoRef         string  `gorm:"index" json:"soRef"`
	Date          string  `json:"date"`
	Amount        float64 `json:"amount"`
	Reason        string  `json:"reason"`
	Status        string  `gorm:"default:'Posted'" json:"status"`
	CreatedBy     string  `json:"createdBy"`
}
