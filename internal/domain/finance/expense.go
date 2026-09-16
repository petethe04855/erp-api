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

// Expense represents an operating or financial expense record.
type Expense struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string          `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Date        string          `gorm:"size:20;index;not null" json:"date"`
	Category    ExpenseCategory `gorm:"size:100;index;not null" json:"category"`
	Channel     ExpenseChannel  `gorm:"size:50;index;not null" json:"channel"`
	Amount      float64         `gorm:"type:numeric(15,2);not null" json:"amount"`
	Vendor      string          `gorm:"size:150" json:"vendor"`
	InvoiceRef  string          `gorm:"size:100" json:"invoice_ref"`
	Description string          `gorm:"size:255" json:"description"`
	CreatedBy   string          `gorm:"size:100" json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
