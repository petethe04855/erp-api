package postgres

import (
	"context"
	"errors"
	"strings"

	"chawy-erp-api/internal/domain/salesreturn"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SalesReturnRepository struct {
	db *gorm.DB
}

func NewSalesReturnRepository(db *gorm.DB) salesreturn.Repository {
	return &SalesReturnRepository{db: db}
}

func (r *SalesReturnRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

func (r *SalesReturnRepository) Create(ctx context.Context, ret *salesreturn.SalesReturn) error {
	return r.getDB(ctx).Create(ret).Error
}

func (r *SalesReturnRepository) FindByID(ctx context.Context, id uint) (*salesreturn.SalesReturn, error) {
	var item salesreturn.SalesReturn
	err := r.getDB(ctx).
		Preload("Lines").
		Where("id = ?", id).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *SalesReturnRepository) FindByIDForUpdate(ctx context.Context, id uint) (*salesreturn.SalesReturn, error) {
	var item salesreturn.SalesReturn
	err := r.getDB(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Lines").
		Where("id = ?", id).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *SalesReturnRepository) FindByReturnNo(ctx context.Context, returnNo string) (*salesreturn.SalesReturn, error) {
	var item salesreturn.SalesReturn
	err := r.getDB(ctx).
		Preload("Lines").
		Where("return_no = ?", returnNo).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *SalesReturnRepository) FindAll(ctx context.Context, query salesreturn.Query) ([]salesreturn.SalesReturn, int64, error) {
	var items []salesreturn.SalesReturn
	var total int64

	db := r.getDB(ctx).Model(&salesreturn.SalesReturn{})

	if query.Search != "" {
		s := "%" + strings.ToLower(query.Search) + "%"
		db = db.Where("LOWER(return_no) LIKE ? OR LOWER(customer_name) LIKE ? OR LOWER(order_no) LIKE ?", s, s, s)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.ReturnType != "" {
		db = db.Where("return_type = ?", query.ReturnType)
	}
	if query.Channel != "" {
		db = db.Where("channel = ?", query.Channel)
	}
	if query.WarehouseID > 0 {
		db = db.Where("warehouse_id = ?", query.WarehouseID)
	}
	if query.StartDate != "" {
		db = db.Where("return_date >= ?", query.StartDate)
	}
	if query.EndDate != "" {
		db = db.Where("return_date <= ?", query.EndDate)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.Limit
	err := db.
		Preload("Lines").
		Order("id DESC").
		Offset(offset).
		Limit(query.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *SalesReturnRepository) Update(ctx context.Context, ret *salesreturn.SalesReturn) error {
	return r.getDB(ctx).Save(ret).Error
}

// UpdateLines persists per-line inspection results (condition/restock) within
// the caller's transaction. It MUST run on the tx connection obtained from the
// context: writing line rows from a pooled connection while the surrounding
// transaction holds their row locks self-deadlocks the request (the outer tx
// waits for the save and the save waits for the outer tx's locks), which is
// exactly the POST /returns/:id/complete hang.
func (r *SalesReturnRepository) UpdateLines(ctx context.Context, lines []salesreturn.SalesReturnLine) error {
	if len(lines) == 0 {
		return nil
	}
	db := r.getDB(ctx)
	for i := range lines {
		if err := db.Save(&lines[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *SalesReturnRepository) DeleteLines(ctx context.Context, returnID uint) error {
	return r.getDB(ctx).Where("return_id = ?", returnID).Delete(&salesreturn.SalesReturnLine{}).Error
}

func (r *SalesReturnRepository) CreateLines(ctx context.Context, lines []salesreturn.SalesReturnLine) error {
	if len(lines) == 0 {
		return nil
	}
	return r.getDB(ctx).Create(&lines).Error
}

// CountReturnedQtyByOrderAndSKU calculates the total quantity of items already claimed in returns
// that are NOT CANCELLED and NOT REJECTED for an order.
func (r *SalesReturnRepository) CountReturnedQtyByOrderAndSKU(ctx context.Context, orderID uint, excludeReturnID uint) (map[string]int, error) {
	type Result struct {
		SKU      string `gorm:"column:sku"`
		TotalQty int    `gorm:"column:total_qty"`
	}

	var results []Result
	query := r.getDB(ctx).
		Table("sales_return_lines srl").
		Select("srl.sku, SUM(srl.quantity) as total_qty").
		Joins("JOIN sales_returns sr ON sr.id = srl.return_id").
		Where("sr.order_id = ? AND sr.status NOT IN (?, ?)", orderID, salesreturn.StatusCancelled, salesreturn.StatusRejected)

	if excludeReturnID > 0 {
		query = query.Where("sr.id != ?", excludeReturnID)
	}

	err := query.Group("srl.sku").Scan(&results).Error
	if err != nil {
		return nil, err
	}

	resMap := make(map[string]int)
	for _, res := range results {
		resMap[strings.ToUpper(strings.TrimSpace(res.SKU))] = res.TotalQty
	}
	return resMap, nil
}
