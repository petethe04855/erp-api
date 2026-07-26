package models

// Expense represents an expense record
type Expense struct {
	ID          uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code        string  `gorm:"uniqueIndex;not null" json:"code"`
	Date        string  `json:"date"`
	Category    string  `json:"category"`
	Channel     string  `json:"channel"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Vendor      string  `json:"vendor"`
	InvoiceRef  string  `json:"invoiceRef"`
	CreatedBy   string  `json:"createdBy"`
}

// MonthBudget represents financial boundaries
type MonthBudget struct {
	ID           uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Year         int     `json:"year"`
	Month        int     `json:"month"`
	Category     string  `json:"category"`
	Channel      string  `json:"channel"`
	BudgetAmount float64 `json:"budgetAmount"`
}
