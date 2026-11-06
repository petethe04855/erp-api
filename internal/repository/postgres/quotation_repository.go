package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	domainQuotation "chawy-erp-api/internal/domain/quotation"
	pkgDatabase "chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QuotationRepository struct {
	db *gorm.DB
}

func NewQuotationRepository(db *gorm.DB) domainQuotation.Repository {
	return &QuotationRepository{db: db}
}

// handle resolves the active DB handle: the transaction inside ctx when one
// is open, otherwise the root db.
func (r *QuotationRepository) handle(ctx context.Context) *gorm.DB {
	return pkgDatabase.GetDBFromContext(ctx, r.db)
}

func (r *QuotationRepository) Create(ctx context.Context, q *domainQuotation.Quotation) error {
	return r.handle(ctx).Create(q).Error
}

func (r *QuotationRepository) FindByID(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	var q domainQuotation.Quotation
	if err := r.handle(ctx).First(&q, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

func (r *QuotationRepository) FindByIDWithLines(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	var q domainQuotation.Quotation
	if err := r.handle(ctx).Preload("Lines").First(&q, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

func (r *QuotationRepository) FindByIDForUpdate(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	var q domainQuotation.Quotation
	err := r.handle(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&q, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

func (r *QuotationRepository) FindByIDForUpdateWithLines(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	var q domainQuotation.Quotation
	err := r.handle(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Lines").First(&q, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

func (r *QuotationRepository) List(ctx context.Context, page, limit int) ([]domainQuotation.Quotation, int64, error) {
	var items []domainQuotation.Quotation
	var total int64

	tx := r.handle(ctx).Model(&domainQuotation.Quotation{})
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListFiltered applies the workspace quotation-list filters.
func (r *QuotationRepository) ListFiltered(ctx context.Context, q domainQuotation.Query) ([]domainQuotation.Quotation, int64, error) {
	var items []domainQuotation.Quotation
	var total int64

	tx := r.handle(ctx).Model(&domainQuotation.Quotation{})

	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("code ILIKE ? OR customer_name ILIKE ?", pat, pat)
	}

	if q.ExpiredOnly {
		today := time.Now().Format("2006-01-02")
		tx = tx.Where("valid_until != '' AND valid_until < ? AND status IN ('Pending', 'Draft', 'Sent', 'Approved')", today)
	} else if q.Status != "" && !strings.EqualFold(q.Status, "all") {
		tx = tx.Where("LOWER(status) = ?", strings.ToLower(q.Status))
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (q.Page - 1) * q.Limit
	if offset < 0 {
		offset = 0
	}

	err := tx.Offset(offset).Limit(q.Limit).Order("id DESC").Find(&items).Error
	return items, total, err
}

func (r *QuotationRepository) Save(ctx context.Context, q *domainQuotation.Quotation) error {
	return r.handle(ctx).Save(q).Error
}