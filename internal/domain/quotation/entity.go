package quotation

import (
	"time"
)

// IsValidStatus reports whether s is one of the allowed quotation statuses.
func IsValidStatus(s Status) bool {
	switch s {
	case StatusDraft, StatusSent, StatusApproved, StatusRejected, StatusConverted:
		return true
	}
	return false
}

// allowedTransitions defines the state machine for quotation status changes.
var allowedTransitions = map[Status][]Status{
	StatusDraft:     {StatusSent, StatusRejected},
	StatusSent:      {StatusApproved, StatusRejected},
	StatusApproved:  {StatusConverted},
	StatusRejected:  {},
	StatusConverted: {},
}

// CanTransition reports whether moving a quotation from current to next is a
// permitted business transition. Converted is terminal and only Approved
// quotations may be converted.
func CanTransition(current, next Status) bool {
	for _, s := range allowedTransitions[current] {
		if s == next {
			return true
		}
	}
	return false
}

// IsValidDate reports whether the value is a YYYY-MM-DD date string.
func IsValidDate(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

type Status string

const (
	StatusDraft     Status = "Draft"
	StatusSent      Status = "Sent"
	StatusApproved  Status = "Approved"
	StatusRejected  Status = "Rejected"
	StatusConverted Status = "Converted"
)

type Quotation struct {
	ID           uint            `json:"id" gorm:"primaryKey"`
	Code         string          `json:"code" gorm:"uniqueIndex;not null;size:100"`
	CustomerID   uint            `json:"customer_id" gorm:"index"`
	CustomerName string          `json:"customer_name" gorm:"size:255"`
	Date         string          `json:"date" gorm:"size:50"`
	ValidUntil   string          `json:"valid_until" gorm:"size:50"`
	LeadSource   string          `json:"lead_source" gorm:"size:100;default:'Manual'"`
	Status       Status          `json:"status" gorm:"size:50;default:'Draft'"`
	TotalAmount  float64         `json:"total_amount" gorm:"type:numeric(12,2);default:0"`
	Note         string          `json:"note" gorm:"size:255"`
	Lines        []QuotationLine `json:"lines" gorm:"foreignKey:QuotationID"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type QuotationLine struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	QuotationID uint      `json:"quotation_id" gorm:"index;not null"`
	ProductID   uint      `json:"product_id"`
	SKU         string    `json:"sku" gorm:"size:100;not null"`
	Name        string    `json:"name" gorm:"size:255"`
	Price       float64   `json:"price" gorm:"type:numeric(12,2);default:0"`
	Quantity    int       `json:"qty" gorm:"not null;default:1"`
	Subtotal    float64   `json:"subtotal" gorm:"type:numeric(12,2);default:0"`
	CreatedAt   time.Time `json:"created_at"`
}
