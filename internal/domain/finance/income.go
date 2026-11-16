package finance

import "time"

type IncomeCategory string

const (
	IncomeCategorySalesLine   IncomeCategory = "ยอดขาย LINE"
	IncomeCategorySalesShopee IncomeCategory = "ยอดขาย Shopee"
	IncomeCategorySalesTikTok IncomeCategory = "ยอดขาย TikTok"
	IncomeCategorySalesStore  IncomeCategory = "ยอดขายหน้าร้าน"
	IncomeCategoryPaymentFee  IncomeCategory = "ค่าธรรมเนียมการชำระเงิน"
	IncomeCategoryOther       IncomeCategory = "รายได้อื่นๆ"
)

// Income represents an income slip record (code prefix INC-).
// Schema MUST stay compatible with the existing `incomes` table created by
// erp-expense-api (same column names/sizes) — the table may already hold data.
type Income struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string         `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Date        string         `gorm:"size:20;index;not null" json:"date"`
	Category    IncomeCategory `gorm:"size:100;index;not null" json:"category"`
	Channel     ExpenseChannel `gorm:"size:50;index;not null" json:"channel"`
	Amount      float64        `gorm:"type:numeric(15,2);not null" json:"amount"`
	ProductName string         `gorm:"size:255;index" json:"product_name"`
	PurchaseURL string         `gorm:"size:500" json:"purchase_url"`
	SlipURL     string         `gorm:"size:500" json:"slip_url"`
	Vendor      string         `gorm:"size:150" json:"vendor"`
	InvoiceRef  string         `gorm:"size:100" json:"invoice_ref"`
	Description string         `gorm:"size:255" json:"description"`
	CreatedBy   string         `gorm:"size:100" json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func (Income) TableName() string { return "incomes" }
