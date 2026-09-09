package postgres

import (
	"context"
	"errors"

	"chawy-erp-api/internal/domain/purchasing"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
)

type PurchasingRepository struct {
	db *gorm.DB
}

func NewPurchasingRepository(db *gorm.DB) purchasing.Repository {
	return &PurchasingRepository{db: db}
}

func (r *PurchasingRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

// Supplier
func (r *PurchasingRepository) CreateSupplier(ctx context.Context, s *purchasing.Supplier) error {
	return r.getDB(ctx).Create(s).Error
}

func (r *PurchasingRepository) FindSupplierByID(ctx context.Context, id uint) (*purchasing.Supplier, error) {
	var s purchasing.Supplier
	err := r.db.WithContext(ctx).First(&s, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *PurchasingRepository) FindAllSuppliers(ctx context.Context, q purchasing.SupplierQuery) ([]purchasing.Supplier, int64, error) {
	var items []purchasing.Supplier
	var total int64

	tx := r.db.WithContext(ctx).Model(&purchasing.Supplier{})

	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("name ILIKE ? OR code ILIKE ? OR contact_person ILIKE ?", pat, pat, pat)
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

func (r *PurchasingRepository) UpdateSupplier(ctx context.Context, s *purchasing.Supplier) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *PurchasingRepository) DeleteSupplier(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&purchasing.Supplier{}, id).Error
}

// Purchase Order
func (r *PurchasingRepository) CreatePO(ctx context.Context, po *purchasing.PurchaseOrder) error {
	return r.getDB(ctx).Create(po).Error
}

func (r *PurchasingRepository) FindPOByID(ctx context.Context, id uint) (*purchasing.PurchaseOrder, error) {
	var po purchasing.PurchaseOrder
	err := r.getDB(ctx).Preload("Items").First(&po, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &po, nil
}

func (r *PurchasingRepository) FindAllPOs(ctx context.Context, q purchasing.POQuery) ([]purchasing.PurchaseOrder, int64, error) {
	var items []purchasing.PurchaseOrder
	var total int64

	tx := r.getDB(ctx).Model(&purchasing.PurchaseOrder{})

	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("po_no ILIKE ? OR supplier_name ILIKE ?", pat, pat)
	}
	if q.SupplierID != 0 {
		tx = tx.Where("supplier_id = ?", q.SupplierID)
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

	err := tx.Preload("Items").Offset(offset).Limit(q.Limit).Order("id DESC").Find(&items).Error
	return items, total, err
}

func (r *PurchasingRepository) UpdatePOStatus(ctx context.Context, id uint, status purchasing.POStatus) error {
	return r.getDB(ctx).Model(&purchasing.PurchaseOrder{}).Where("id = ?", id).Update("status", status).Error
}

func (r *PurchasingRepository) UpdatePO(ctx context.Context, po *purchasing.PurchaseOrder) error {
	return r.getDB(ctx).Save(po).Error
}
