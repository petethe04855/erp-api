package postgres

import (
	"context"
	"errors"

	"chawy-erp-api/internal/domain/stock"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StockRepository struct {
	db *gorm.DB
}

func NewStockRepository(db *gorm.DB) stock.Repository {
	return &StockRepository{db: db}
}

func (r *StockRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

func (r *StockRepository) GetBySKUID(ctx context.Context, skuID, warehouseID uint) (*stock.Stock, error) {
	var item stock.Stock
	err := r.getDB(ctx).
		Where("sku_id = ? AND warehouse_id = ?", skuID, warehouseID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// GetBySKUIDForUpdate locks the stock row with SELECT ... FOR UPDATE so the
// availability check and quantity update happen under the same lock (FULL-07).
func (r *StockRepository) GetBySKUIDForUpdate(ctx context.Context, skuID, warehouseID uint) (*stock.Stock, error) {
	var item stock.Stock
	err := r.getDB(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("sku_id = ? AND warehouse_id = ?", skuID, warehouseID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *StockRepository) FindAll(ctx context.Context, q stock.Query) ([]stock.Stock, int64, error) {
	var items []stock.Stock
	var total int64

	tx := r.getDB(ctx).Model(&stock.Stock{})

	if q.SKUID != 0 {
		tx = tx.Where("sku_id = ?", q.SKUID)
	}

	if q.WarehouseID != 0 {
		tx = tx.Where("warehouse_id = ?", q.WarehouseID)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (q.Page - 1) * q.Limit
	if offset < 0 {
		offset = 0
	}

	err := tx.Offset(offset).Limit(q.Limit).Order("id DESC").Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *StockRepository) UpdateQuantity(ctx context.Context, skuID, warehouseID uint, delta int) (*stock.Stock, error) {
	var item stock.Stock

	db := r.getDB(ctx)
	// If already in transaction, do not nest with new uncoordinated transaction
	updateFn := func(tx *gorm.DB) error {
		// Lock the row with FOR UPDATE
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("sku_id = ? AND warehouse_id = ?", skuID, warehouseID).
			First(&item).Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Initialize new stock row if not existing
				item = stock.Stock{
					SKUID:        skuID,
					WarehouseID:  warehouseID,
					Quantity:     delta,
					AvailableQty: delta,
				}
				return tx.Create(&item).Error
			}
			return err
		}

		item.Quantity += delta
		item.AvailableQty = item.Quantity - item.ReservedQty
		return tx.Save(&item).Error
	}

	var err error
	// Check if already in transaction (SavePoint or regular)
	err = db.Transaction(updateFn)
	if err != nil {
		return nil, err
	}

	return &item, nil
}

func (r *StockRepository) ReserveStock(ctx context.Context, skuID, warehouseID uint, qty int) (*stock.Stock, error) {
	var item stock.Stock
	db := r.getDB(ctx)
	reserveFn := func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("sku_id = ? AND warehouse_id = ?", skuID, warehouseID).
			First(&item).Error
		if err != nil {
			return err
		}
		item.ReservedQty += qty
		item.AvailableQty = item.Quantity - item.ReservedQty
		return tx.Save(&item).Error
	}
	if err := db.Transaction(reserveFn); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *StockRepository) ReleaseStock(ctx context.Context, skuID, warehouseID uint, qty int) (*stock.Stock, error) {
	var item stock.Stock
	db := r.getDB(ctx)
	releaseFn := func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("sku_id = ? AND warehouse_id = ?", skuID, warehouseID).
			First(&item).Error
		if err != nil {
			return err
		}
		item.ReservedQty -= qty
		if item.ReservedQty < 0 {
			item.ReservedQty = 0
		}
		item.AvailableQty = item.Quantity - item.ReservedQty
		return tx.Save(&item).Error
	}
	if err := db.Transaction(releaseFn); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *StockRepository) CreateMovement(ctx context.Context, movement *stock.StockMovement) error {
	return r.getDB(ctx).Create(movement).Error
}

func (r *StockRepository) GetMovements(ctx context.Context, skuID uint, page, limit int) ([]stock.StockMovement, int64, error) {
	var movements []stock.StockMovement
	var total int64

	tx := r.db.WithContext(ctx).Model(&stock.StockMovement{})
	if skuID != 0 {
		tx = tx.Where("sku_id = ?", skuID)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	err := tx.Offset(offset).Limit(limit).Order("id DESC").Find(&movements).Error
	if err != nil {
		return nil, 0, err
	}

	return movements, total, nil
}
