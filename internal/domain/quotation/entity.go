package quotation

import (
	"time"
)

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
