package finance

import (
	"time"
)

type AccountType string

const (
	AccountTypeAsset     AccountType = "Asset"
	AccountTypeLiability AccountType = "Liability"
	AccountTypeEquity    AccountType = "Equity"
	AccountTypeRevenue   AccountType = "Revenue"
	AccountTypeExpense   AccountType = "Expense"
)

// Account represents a Chart of Accounts entry.
type Account struct {
	ID             uint        `gorm:"primaryKey;autoIncrement" json:"id"`
	Code           string      `gorm:"size:20;uniqueIndex;not null" json:"code"`
	Name           string      `gorm:"size:150;not null" json:"name"`
	Type           AccountType `gorm:"size:30;not null" json:"type"`
	IsActive       bool        `gorm:"default:true" json:"is_active"`
	OpeningBalance float64     `gorm:"type:numeric(15,2);default:0" json:"opening_balance"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// AccountMapping maps business event keys to standard account codes.
type AccountMapping struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	MappingKey  string    `gorm:"size:80;uniqueIndex;not null" json:"mapping_key"`
	AccountCode string    `gorm:"size:20;not null;index" json:"account_code"`
	Description string    `gorm:"size:255" json:"description"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
