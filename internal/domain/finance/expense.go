package finance

import (
	"time"
)

type ExpenseCategory string

const (
	ExpenseCategoryAds         ExpenseCategory = "ค่าโฆษณา"
	ExpenseCategoryPlatformFee ExpenseCategory = "ค่าธรรมเนียมแพลตฟอร์ม"
	ExpenseCategoryCOGS        ExpenseCategory = "ต้นทุนขาย/วัตถุดิบ"
	ExpenseCategorySGA         ExpenseCategory = "ค่าใช้จ่ายในการบริหาร"
	ExpenseCategoryShipping    ExpenseCategory = "ค่าขนส่ง"
	ExpenseCategorySalary      ExpenseCategory = "ค่าแรง/เงินเดือน"
	ExpenseCategoryOther       ExpenseCategory = "อื่นๆ"
)

type ExpenseChannel string

const (
	ExpenseChannelTikTok  ExpenseChannel = "TikTok"
	ExpenseChannelShopee  ExpenseChannel = "Shopee"
	ExpenseChannelLine    ExpenseChannel = "LINE"
	ExpenseChannelManual  ExpenseChannel = "Manual"
	ExpenseChannelGeneral ExpenseChannel = "ทั่วไป"
)

type ExpenseStatus string

const (
	// ExpenseStatusActive is a live expense whose journal entry counts in reports.
	ExpenseStatusActive ExpenseStatus = "Active"
	// ExpenseStatusVoided is a cancelled expense. Its journal entry has been
	// reversed; the record is kept for the audit trail instead of being deleted.
	ExpenseStatusVoided ExpenseStatus = "Voided"
)

// Expense represents an operating or financial expense record.
type Expense struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string          `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Date        string          `gorm:"size:20;index;not null" json:"date"`
	Category    ExpenseCategory `gorm:"size:100;index;not null" json:"category"`
	Channel     ExpenseChannel  `gorm:"size:50;index;not null" json:"channel"`
	Amount      float64         `gorm:"type:numeric(15,2);not null" json:"amount"`
	ProductName string          `gorm:"size:255;index" json:"product_name"`
	PurchaseURL string          `gorm:"size:500" json:"purchase_url"`
	SlipURL     string          `gorm:"size:500" json:"slip_url"`
	Vendor      string          `gorm:"size:150" json:"vendor"`
	InvoiceRef  string          `gorm:"size:100" json:"invoice_ref"`
	Description string          `gorm:"size:255" json:"description"`
	CreatedBy   string          `gorm:"size:100" json:"created_by"`
	Status      ExpenseStatus   `gorm:"size:20;index;not null;default:'Active'" json:"status"`
	VoidedAt    *time.Time      `json:"voided_at,omitempty"`
	VoidedBy    string          `gorm:"size:100" json:"voided_by,omitempty"`
	VoidReason  string          `gorm:"size:255" json:"void_reason,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
