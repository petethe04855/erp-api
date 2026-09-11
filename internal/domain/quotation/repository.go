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
