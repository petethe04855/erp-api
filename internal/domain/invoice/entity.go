package invoice

import (
	"context"
	"time"
)

type Status string

const (
	StatusUnpaid        Status = "UNPAID"
	StatusPartiallyPaid Status = "PARTIALLY_PAID"
	StatusPaid          Status = "PAID"
	StatusCancelled     Status = "CANCELLED"
)

type Invoice struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	InvoiceNo string `json:"invoice_no" gorm:"uniqueIndex;not null;size:100"`
	// FULL-20: unique index enforces one-invoice-per-order at the DB level.
	// Postgres allows multiple NULLs, so non-order invoices are unaffected.
	OrderID       *uint      `json:"order_id" gorm:"uniqueIndex"`
	OrderNo       string     `json:"order_no" gorm:"size:100"`
	CustomerID    uint       `json:"customer_id" gorm:"index"`
	CustomerName  string     `json:"customer_name" gorm:"size:255"`
	Amount        float64    `json:"amount" gorm:"type:numeric(12,2);not null;default:0"`
	PaidAmount    float64    `json:"paid_amount" gorm:"type:numeric(12,2);default:0"`
	Status        Status     `json:"status" gorm:"size:50;default:'UNPAID'"`
	PaymentMethod string     `json:"payment_method" gorm:"size:50"`
	DueDate       *time.Time `json:"due_date"`
	PaidAt        *time.Time `json:"paid_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Query struct {
	Search    string
	Status    string
	StartDate string
	EndDate   string
	Page      int
	Limit     int
}

type Repository interface {
	Create(ctx context.Context, inv *Invoice) error
	FindByID(ctx context.Context, id uint) (*Invoice, error)
	// FindByIDForUpdate locks the invoice row (SELECT ... FOR UPDATE) so
	// concurrent payments cannot read the same PaidAmount snapshot.
	FindByIDForUpdate(ctx context.Context, id uint) (*Invoice, error)
	FindByOrderID(ctx context.Context, orderID uint) (*Invoice, error)
	FindByInvoiceNo(ctx context.Context, no string) (*Invoice, error)
	FindAll(ctx context.Context, q Query) ([]Invoice, int64, error)
	Update(ctx context.Context, inv *Invoice) error
}
