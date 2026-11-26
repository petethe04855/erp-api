package postgres

import (
	"context"
	"errors"
	"time"

	domainStockOut "chawy-erp-api/internal/domain/stockout"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockOutRepository stores the stock-out count config and batches.
type StockOutRepository struct {
	db *gorm.DB
}

// NewStockOutRepository builds the repository.
func NewStockOutRepository(db *gorm.DB) domainStockOut.Repository {
	return &StockOutRepository{db: db}
}

func (r *StockOutRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

// GetConfig returns nil, nil before the first save.
func (r *StockOutRepository) GetConfig(ctx context.Context) (*domainStockOut.ConfigRecord, error) {
	var rec domainStockOut.ConfigRecord
	if err := r.getDB(ctx).Where("id = ?", domainStockOut.ConfigRowID).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// SaveConfig upserts the single config row.
func (r *StockOutRepository) SaveConfig(ctx context.Context, rec *domainStockOut.ConfigRecord) error {
	return r.getDB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "updated_by", "updated_at"}),
	}).Create(rec).Error
}

func (r *StockOutRepository) FindByClientID(ctx context.Context, clientID string) (*domainStockOut.Batch, error) {
	var b domainStockOut.Batch
	if err := r.getDB(ctx).Where("client_id = ?", clientID).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

func (r *StockOutRepository) FindByIDForUpdate(ctx context.Context, id uint) (*domainStockOut.Batch, error) {
	var b domainStockOut.Batch
	if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

func (r *StockOutRepository) Create(ctx context.Context, b *domainStockOut.Batch) error {
	return r.getDB(ctx).Create(b).Error
}

func (r *StockOutRepository) SetBatchNo(ctx context.Context, id uint, no string) error {
	return r.getDB(ctx).Model(&domainStockOut.Batch{}).Where("id = ?", id).Update("batch_no", no).Error
}

func (r *StockOutRepository) MarkVoided(ctx context.Context, id uint, by string, at time.Time) error {
	return r.getDB(ctx).Model(&domainStockOut.Batch{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": domainStockOut.StatusVoided, "voided_at": at, "voided_by": by,
	}).Error
}

func (r *StockOutRepository) MarkReviewed(ctx context.Context, id uint, by, note string, at time.Time) error {
	return r.getDB(ctx).Model(&domainStockOut.Batch{}).Where("id = ?", id).Updates(map[string]interface{}{
		"reviewed_at": at, "reviewed_by": by, "review_note": note,
	}).Error
}

func (r *StockOutRepository) List(ctx context.Context, f domainStockOut.Filter) ([]domainStockOut.Batch, error) {
	var out []domainStockOut.Batch
	q := r.getDB(ctx).Where("counted_at >= ? AND counted_at < ?", f.From, f.To).Order("counted_at ASC, id ASC")
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
