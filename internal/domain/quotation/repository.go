package quotation

import "context"

// Repository is the persistence contract for quotations. Implementations live
// in internal/repository/postgres; the usecase layer depends only on this.
type Repository interface {
	Create(ctx context.Context, q *Quotation) error
	FindByID(ctx context.Context, id uint) (*Quotation, error)
	// FindByIDWithLines loads the quotation with its lines preloaded.
	FindByIDWithLines(ctx context.Context, id uint) (*Quotation, error)
	// FindByIDForUpdate locks the quotation row (SELECT ... FOR UPDATE) so
	// concurrent status updates / conversions serialize correctly.
	FindByIDForUpdate(ctx context.Context, id uint) (*Quotation, error)
	// FindByIDForUpdateWithLines locks the quotation row and loads lines in
	// the same transaction.
	FindByIDForUpdateWithLines(ctx context.Context, id uint) (*Quotation, error)
	List(ctx context.Context, page, limit int) ([]Quotation, int64, error)
	Save(ctx context.Context, q *Quotation) error
}

// LineInput is a single quotation line as submitted by a client before
// validation and price/qty enforcement.
type LineInput struct {
	SKU       string
	Name      string
	Price     float64
	Qty       int
	ProductID uint
}

// CreateInput is the validated business input for creating a quotation.
type CreateInput struct {
	Customer   string
	Date       string
	ValidUntil string
	Status     string
	LeadSource string
	Note       string
	Lines      []LineInput
}

// ConversionResult reports the sales order created from a quotation.
type ConversionResult struct {
	QuotationID uint
	OrderID     uint
	OrderNo     string
}
