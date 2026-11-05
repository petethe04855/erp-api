package postgres

import (
	"context"
	"errors"
	"strings"

	"chawy-erp-api/internal/domain/order"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) order.Repository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

func (r *OrderRepository) Create(ctx context.Context, o *order.Order) error {
	return r.getDB(ctx).Create(o).Error
}

func (r *OrderRepository) FindByID(ctx context.Context, id uint) (*order.Order, error) {
	var o order.Order
	err := r.getDB(ctx).Preload("Items").First(&o, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

// FindByIDForUpdate locks the order row with SELECT ... FOR UPDATE so status
// transitions cannot race (FULL-07). Must be called inside a transaction.
func (r *OrderRepository) FindByIDForUpdate(ctx context.Context, id uint) (*order.Order, error) {
	var o order.Order
	err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").First(&o, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) FindByOrderNo(ctx context.Context, orderNo string) (*order.Order, error) {
	var o order.Order
	err := r.getDB(ctx).Preload("Items").Where("order_no = ?", orderNo).First(&o).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) FindAll(ctx context.Context, q order.Query) ([]order.Order, int64, error) {
	var items []order.Order
	var total int64

	tx := r.getDB(ctx).Model(&order.Order{})

	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("order_no ILIKE ? OR customer_name ILIKE ?", pat, pat)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Channel != "" {
		tx = tx.Where("channel = ?", q.Channel)
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

func (r *OrderRepository) UpdateStatus(ctx context.Context, id uint, status order.Status) error {
	return r.getDB(ctx).Model(&order.Order{}).Where("id = ?", id).Update("status", status).Error
}

// FindByRef resolves an order by numeric id or document number, mirroring the
// lookup the workspace sales screens perform.
func (r *OrderRepository) FindByRef(ctx context.Context, ref string) (*order.Order, error) {
	var o order.Order
	err := r.getDB(ctx).Preload("Items").Where("id::text = ? OR order_no = ?", ref, ref).First(&o).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

// FindAllForSales applies the sales-list filters used by the workspace screen.
func (r *OrderRepository) FindAllForSales(ctx context.Context, q order.SalesQuery) ([]order.Order, int64, error) {
	var items []order.Order
	var total int64

	tx := r.getDB(ctx).Model(&order.Order{})

	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("order_no ILIKE ? OR customer_name ILIKE ?", pat, pat)
	}

	if q.Status != "" && !strings.EqualFold(q.Status, "all") {
		stUpper := strings.ToUpper(q.Status)
		if stUpper == "COMPLETED" {
			tx = tx.Where("UPPER(status) IN ('COMPLETED', 'SHIPPED')")
		} else {
			tx = tx.Where("UPPER(status) = ?", stUpper)
		}
	}

	if q.Channel != "" && !strings.EqualFold(q.Channel, "all") {
		cLower := strings.ToLower(q.Channel)
		if cLower == "manual" {
			tx = tx.Where("LOWER(channel) IN (?) OR channel IS NULL OR channel = ''", []string{"manual", "direct"})
		} else {
			tx = tx.Where("LOWER(channel) LIKE ?", "%"+cLower+"%")
		}
	}

	if strings.EqualFold(q.PaymentStatus, "paid") {
		// Paid orders have an invoice with status PAID.
		tx = tx.Where("id IN (SELECT order_id FROM invoices WHERE order_id IS NOT NULL AND status = 'PAID')")
	} else if strings.EqualFold(q.PaymentStatus, "unpaid") {
		// Unpaid orders have either no invoice or an unpaid invoice.
		tx = tx.Where("id NOT IN (SELECT order_id FROM invoices WHERE order_id IS NOT NULL AND status = 'PAID')")
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

func (r *OrderRepository) Update(ctx context.Context, o *order.Order) error {
	return r.getDB(ctx).Save(o).Error
}
