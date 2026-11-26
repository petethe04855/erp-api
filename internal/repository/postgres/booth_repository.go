package postgres

import (
	"context"
	"errors"
	"time"

	domainBooth "chawy-erp-api/internal/domain/booth"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BoothRepository stores the booth config and bills.
type BoothRepository struct {
	db *gorm.DB
}

// NewBoothRepository builds the repository.
func NewBoothRepository(db *gorm.DB) domainBooth.Repository {
	return &BoothRepository{db: db}
}

func (r *BoothRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

// GetConfig returns nil, nil before the first save.
func (r *BoothRepository) GetConfig(ctx context.Context) (*domainBooth.ConfigRecord, error) {
	var rec domainBooth.ConfigRecord
	err := r.getDB(ctx).Where("id = ?", domainBooth.ConfigRowID).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// SaveConfig upserts the single config row.
func (r *BoothRepository) SaveConfig(ctx context.Context, rec *domainBooth.ConfigRecord) error {
	return r.getDB(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"data", "updated_by", "updated_at"}),
	}).Create(rec).Error
}

func (r *BoothRepository) FindByClientRef(ctx context.Context, ref string) (*domainBooth.Sale, error) {
	var s domainBooth.Sale
	err := r.getDB(ctx).Where("client_ref = ?", ref).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *BoothRepository) FindByIDForUpdate(ctx context.Context, id uint) (*domainBooth.Sale, error) {
	var s domainBooth.Sale
	err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *BoothRepository) Create(ctx context.Context, s *domainBooth.Sale) error {
	return r.getDB(ctx).Create(s).Error
}

func (r *BoothRepository) SetSaleNo(ctx context.Context, id uint, no string) error {
	return r.getDB(ctx).Model(&domainBooth.Sale{}).Where("id = ?", id).Update("sale_no", no).Error
}

func (r *BoothRepository) MarkVoided(ctx context.Context, id uint, by, reason string, at time.Time) error {
	return r.getDB(ctx).Model(&domainBooth.Sale{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": domainBooth.StatusVoided, "voided_at": at, "voided_by": by, "void_reason": reason,
	}).Error
}

func (r *BoothRepository) List(ctx context.Context, f domainBooth.SaleFilter) ([]domainBooth.Sale, error) {
	var out []domainBooth.Sale
	q := r.getDB(ctx).Where("created_at >= ? AND created_at < ?", f.From, f.To).Order("created_at ASC, id ASC")
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
