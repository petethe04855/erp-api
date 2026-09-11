package order

import (
	"context"
	"time"
)

type Status string

const (
	StatusPending   Status = "PENDING"
	StatusConfirmed Status = "CONFIRMED"
	StatusShipped   Status = "SHIPPED"
	StatusCancelled Status = "CANCELLED"
)

func IsValidStatus(s Status) bool {
	switch s {
	case StatusPending, StatusConfirmed, StatusShipped, StatusCancelled:
		return true
	default:
		return false
	}
}

func CanTransition(from, to Status) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusPending:
		return to == StatusConfirmed || to == StatusCancelled
	case StatusConfirmed:
		return to == StatusShipped || to == StatusCancelled
	case StatusShipped:
		// Terminal for order status updates (cannot cancel or revert)
		return false
	case StatusCancelled:
		// Terminal status
		return false
	default:
		return false
	}
}

type Order struct {
	ID           uint        `json:"id" gorm:"primaryKey"`
	OrderNo      string      `json:"order_no" gorm:"uniqueIndex;not null;size:100"`
	CustomerID   uint        `json:"customer_id" gorm:"index"`
	CustomerName string      `json:"customer_name" gorm:"size:255"`
	Channel      string      `json:"channel" gorm:"size:50;default:'direct'"` // direct, tiktok, shopee
	Status       Status      `json:"status" gorm:"size:50;default:'PENDING'"`
	TotalAmount  float64     `json:"total_amount" gorm:"type:numeric(12,2);default:0"`
	Note         string      `json:"note" gorm:"size:255"`
	Items        []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	OrderID   uint      `json:"order_id" gorm:"index;not null"`
	SKU       string    `json:"sku" gorm:"size:100;not null"`
	Name      string    `json:"name" gorm:"size:255"`
	Price     float64   `json:"price" gorm:"type:numeric(12,2);default:0"`
	Quantity  int       `json:"quantity" gorm:"not null;default:1"`
	Subtotal  float64   `json:"subtotal" gorm:"type:numeric(12,2);default:0"`
	CreatedAt time.Time `json:"created_at"`
}

type Query struct {
	Search    string
	Status    string
	Channel   string
	StartDate string
	EndDate   string
	Page      int
	Limit     int
}

type Repository interface {
	Create(ctx context.Context, order *Order) error
	FindByID(ctx context.Context, id uint) (*Order, error)
	// FindByIDForUpdate locks the order row (SELECT ... FOR UPDATE) to prevent
	// concurrent ship/cancel races.
	FindByIDForUpdate(ctx context.Context, id uint) (*Order, error)
	FindByOrderNo(ctx context.Context, orderNo string) (*Order, error)
	FindAll(ctx context.Context, query Query) ([]Order, int64, error)
	UpdateStatus(ctx context.Context, id uint, status Status) error
	Update(ctx context.Context, order *Order) error
}
