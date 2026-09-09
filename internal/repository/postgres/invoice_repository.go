package postgres

import (
	"context"
	"errors"

	"chawy-erp-api/internal/domain/invoice"

	"gorm.io/gorm"
)

type InvoiceRepository struct {
	db *gorm.DB
}

func NewInvoiceRepository(db *gorm.DB) invoice.Repository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Create(ctx context.Context, inv *invoice.Invoice) error {
	return r.db.WithContext(ctx).Create(inv).Error
}

func (r *InvoiceRepository) FindByID(ctx context.Context, id uint) (*invoice.Invoice, error) {
	var inv invoice.Invoice
	err := r.db.WithContext(ctx).First(&inv, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}

func (r *InvoiceRepository) FindByOrderID(ctx context.Context, orderID uint) (*invoice.Invoice, error) {
	var inv invoice.Invoice
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&inv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}

func (r *InvoiceRepository) FindByInvoiceNo(ctx context.Context, no string) (*invoice.Invoice, error) {
	var inv invoice.Invoice
	err := r.db.WithContext(ctx).Where("invoice_no = ?", no).First(&inv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}

func (r *InvoiceRepository) FindAll(ctx context.Context, q invoice.Query) ([]invoice.Invoice, int64, error) {
	var items []invoice.Invoice
	var total int64

	tx := r.db.WithContext(ctx).Model(&invoice.Invoice{})

	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("invoice_no ILIKE ? OR order_no ILIKE ? OR customer_name ILIKE ?", pat, pat, pat)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
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

func (r *InvoiceRepository) Update(ctx context.Context, inv *invoice.Invoice) error {
	return r.db.WithContext(ctx).Save(inv).Error
}
