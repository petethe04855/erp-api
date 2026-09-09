package postgres

import (
	"context"
	"errors"

	"chawy-erp-api/internal/domain/customer"

	"gorm.io/gorm"
)

type CustomerRepository struct {
	db *gorm.DB
}

func NewCustomerRepository(db *gorm.DB) customer.Repository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) Create(ctx context.Context, c *customer.Customer) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *CustomerRepository) FindByID(ctx context.Context, id uint) (*customer.Customer, error) {
	var c customer.Customer
	err := r.db.WithContext(ctx).First(&c, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *CustomerRepository) FindByCode(ctx context.Context, code string) (*customer.Customer, error) {
	var c customer.Customer
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *CustomerRepository) FindAll(ctx context.Context, q customer.Query) ([]customer.Customer, int64, error) {
	var items []customer.Customer
	var total int64

	tx := r.db.WithContext(ctx).Model(&customer.Customer{})

	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("name ILIKE ? OR phone ILIKE ? OR email ILIKE ? OR code ILIKE ?", pat, pat, pat, pat)
	}
	if q.Channel != "" {
		tx = tx.Where("channel = ?", q.Channel)
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

func (r *CustomerRepository) Update(ctx context.Context, c *customer.Customer) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *CustomerRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&customer.Customer{}, id).Error
}
