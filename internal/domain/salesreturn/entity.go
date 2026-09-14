package salesreturn

import (
	"context"
	"time"
)

type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusSubmitted Status = "SUBMITTED"
	StatusApproved  Status = "APPROVED"
	StatusRejected  Status = "REJECTED"
	StatusCompleted Status = "COMPLETED"
	StatusCancelled Status = "CANCELLED"
)

type ReturnType string

const (
	ReturnTypeCustomer ReturnType = "CUSTOMER"
	ReturnTypeInternal ReturnType = "INTERNAL"
)

type ItemCondition string

const (
	ConditionGood      ItemCondition = "GOOD"
	ConditionDamaged   ItemCondition = "DAMAGED"
	ConditionExpired   ItemCondition = "EXPIRED"
	ConditionWrongItem ItemCondition = "WRONG_ITEM"
)

type ReasonCode string

const (
	ReasonCustomerChange ReasonCode = "CUSTOMER_CHANGE"
	ReasonDefect         ReasonCode = "DEFECT"
	ReasonLateDelivery   ReasonCode = "LATE_DELIVERY"
	ReasonWrongItem      ReasonCode = "WRONG_ITEM"
	ReasonOther          ReasonCode = "OTHER"
)

func IsValidStatus(s Status) bool {
	switch s {
	case StatusDraft, StatusSubmitted, StatusApproved, StatusRejected, StatusCompleted, StatusCancelled:
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
	case StatusDraft:
		return to == StatusSubmitted || to == StatusCancelled
	case StatusSubmitted:
		return to == StatusApproved || to == StatusRejected || to == StatusCancelled
	case StatusApproved:
		return to == StatusCompleted || to == StatusCancelled
	case StatusRejected, StatusCompleted, StatusCancelled:
		// Terminal statuses
		return false
	default:
		return false
	}
}

type SalesReturn struct {
	ID                 uint              `json:"id" gorm:"primaryKey"`
	ReturnNo           string            `json:"return_no" gorm:"uniqueIndex;not null;size:100"`
	ReturnType         ReturnType        `json:"return_type" gorm:"size:50;default:'CUSTOMER'"`
	OrderID            *uint             `json:"order_id" gorm:"index"`
	OrderNo            string            `json:"order_no" gorm:"size:100"`
	InvoiceID          *uint             `json:"invoice_id"`
	InvoiceNo          string            `json:"invoice_no" gorm:"size:100"`
	CustomerID         *uint             `json:"customer_id" gorm:"index"`
	CustomerName       string            `json:"customer_name" gorm:"size:255"`
	Channel            string            `json:"channel" gorm:"size:50;default:'direct'"`
	WarehouseID        uint              `json:"warehouse_id" gorm:"default:1"`
	ReturnDate         time.Time         `json:"return_date"`
	Status             Status            `json:"status" gorm:"size:50;default:'DRAFT'"`
	Reason             string            `json:"reason" gorm:"size:255"`
	Note               string            `json:"note" gorm:"size:255"`
	CancellationReason string            `json:"cancellation_reason" gorm:"size:255"`
	TotalQty           int               `json:"total_qty" gorm:"default:0"`
	Subtotal           float64           `json:"subtotal" gorm:"type:numeric(12,2);default:0"`
	VatAdjust          float64           `json:"vat_adjust" gorm:"type:numeric(12,2);default:0"`
	NetAmount          float64           `json:"net_amount" gorm:"type:numeric(12,2);default:0"`
	CreatedBy          string            `json:"created_by" gorm:"size:100"`
	ApprovedBy         string            `json:"approved_by" gorm:"size:100"`
	CompletedBy        string            `json:"completed_by" gorm:"size:100"`
	Lines              []SalesReturnLine `json:"lines" gorm:"foreignKey:ReturnID"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type SalesReturnLine struct {
	ID         uint          `json:"id" gorm:"primaryKey"`
	ReturnID   uint          `json:"return_id" gorm:"index;not null"`
	SKU        string        `json:"sku" gorm:"size:100;not null"`
	Name       string        `json:"name" gorm:"size:255"`
	OrderedQty int           `json:"ordered_qty" gorm:"default:0"`
	Quantity   int           `json:"quantity" gorm:"not null;default:1"`
	UnitPrice  float64       `json:"unit_price" gorm:"type:numeric(12,2);default:0"`
	LineAmount float64       `json:"line_amount" gorm:"type:numeric(12,2);default:0"`
	Condition  ItemCondition `json:"condition" gorm:"size:50;default:'GOOD'"`
	Restock    bool          `json:"restock" gorm:"default:true"`
	ReasonCode ReasonCode    `json:"reason_code" gorm:"size:50;default:'CUSTOMER_CHANGE'"`
	LotRef     string        `json:"lot_ref" gorm:"size:100"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

type Query struct {
	Search      string
	Status      string
	ReturnType  string
	Channel     string
	WarehouseID uint
	StartDate   string
	EndDate     string
	Page        int
	Limit       int
}

type ReturnableItem struct {
	SKU              string  `json:"sku"`
	Name             string  `json:"name"`
	OrderedQty       int     `json:"ordered_qty"`
	ReturnedQtySoFar int     `json:"returned_qty_so_far"`
	ReturnableQty    int     `json:"returnable_qty"`
	UnitPrice        float64 `json:"unit_price"`
}

type Repository interface {
	Create(ctx context.Context, ret *SalesReturn) error
	FindByID(ctx context.Context, id uint) (*SalesReturn, error)
	FindByIDForUpdate(ctx context.Context, id uint) (*SalesReturn, error)
	FindByReturnNo(ctx context.Context, returnNo string) (*SalesReturn, error)
	FindAll(ctx context.Context, query Query) ([]SalesReturn, int64, error)
	Update(ctx context.Context, ret *SalesReturn) error
	UpdateLines(ctx context.Context, lines []SalesReturnLine) error
	DeleteLines(ctx context.Context, returnID uint) error
	CreateLines(ctx context.Context, lines []SalesReturnLine) error
	CountReturnedQtyByOrderAndSKU(ctx context.Context, orderID uint, excludeReturnID uint) (map[string]int, error)
}
