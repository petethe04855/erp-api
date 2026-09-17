package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

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

func (r *StockRepository) FindAllBySKU(ctx context.Context, q stock.StockBySKUQuery) ([]stock.StockBySKU, int64, error) {
	type AggregateResult struct {
		SKUID          uint   `gorm:"column:sku_id"`
		SKUCode        string `gorm:"column:sku_code"`
		Quantity       int    `gorm:"column:quantity"`
		ReservedQty    int    `gorm:"column:reserved_qty"`
		AvailableQty   int    `gorm:"column:available_qty"`
		WarehouseCount int    `gorm:"column:warehouse_count"`
	}

	db := r.getDB(ctx)
	tx := db.Table("stocks").
		Select("stocks.sku_id, stocks.sku_code, COALESCE(SUM(stocks.quantity), 0) AS quantity, COALESCE(SUM(stocks.reserved_qty), 0) AS reserved_qty, COALESCE(SUM(stocks.available_qty), 0) AS available_qty, COUNT(DISTINCT stocks.warehouse_id) AS warehouse_count").
		Joins("JOIN skus ON skus.id = stocks.sku_id").
		Where("skus.status = 'active'")

	if q.Search != "" {
		s := "%" + q.Search + "%"
		tx = tx.Where("stocks.sku_code ILIKE ? OR skus.name ILIKE ?", s, s)
	}

	tx = tx.Group("stocks.sku_id, stocks.sku_code")

	// Count total aggregated SKU groups
	var total int64
	countTx := db.Table("(?) as agg", tx).Count(&total)
	if countTx.Error != nil {
		return nil, 0, countTx.Error
	}

	queryTx := tx.Order("stocks.sku_code ASC")
	if q.Limit > 0 {
		offset := (q.Page - 1) * q.Limit
		if offset < 0 {
			offset = 0
		}
		queryTx = queryTx.Offset(offset).Limit(q.Limit)
	}

	var rawResults []AggregateResult
	if err := queryTx.Scan(&rawResults).Error; err != nil {
		return nil, 0, err
	}

	results := make([]stock.StockBySKU, len(rawResults))
	for i, rItem := range rawResults {
		results[i] = stock.StockBySKU{
			SKUID:          rItem.SKUID,
			SKUCode:        rItem.SKUCode,
			Quantity:       rItem.Quantity,
			ReservedQty:    rItem.ReservedQty,
			AvailableQty:   rItem.AvailableQty,
			WarehouseCount: rItem.WarehouseCount,
		}
	}

	return results, total, nil
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
	type rawMovement struct {
		ID                uint               `gorm:"column:id"`
		SKUID             uint               `gorm:"column:sku_id"`
		SKUCode           string             `gorm:"column:sku_code"`
		WarehouseID       uint               `gorm:"column:warehouse_id"`
		StockLotID        *uint              `gorm:"column:stock_lot_id"`
		SourceFormulaCode string             `gorm:"column:source_formula_code"`
		Channel           string             `gorm:"column:channel"`
		Type              stock.MovementType `gorm:"column:type"`
		Quantity          int                `gorm:"column:quantity"`
		BeforeQty         int                `gorm:"column:before_qty"`
		AfterQty          int                `gorm:"column:after_qty"`
		ReferenceType     string             `gorm:"column:reference_type"`
		ReferenceID       string             `gorm:"column:reference_id"`
		Note              string             `gorm:"column:note"`
		CreatedAt         time.Time          `gorm:"column:created_at"`
		LotNumber         *string            `gorm:"column:lot_number"`
		SupplierLot       *string            `gorm:"column:supplier_lot"`
		ExpiryDate        *string            `gorm:"column:expiry_date"`
	}

	tx := r.db.WithContext(ctx).Table("stock_movements sm").
		Select(`
			sm.id,
			sm.sku_id,
			sm.sku_code,
			sm.warehouse_id,
			sm.stock_lot_id,
			sm.source_formula_code,
			sm.channel,
			sm.type,
			sm.quantity,
			sm.before_qty,
			sm.after_qty,
			sm.reference_type,
			sm.reference_id,
			sm.note,
			sm.created_at,
			sl.lot_number,
			sl.supplier_lot,
			sl.expiry_date
		`).
		Joins("LEFT JOIN stock_lots sl ON sm.stock_lot_id = sl.id")

	if skuID != 0 {
		tx = tx.Where("sm.sku_id = ?", skuID)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	var raws []rawMovement
	err := tx.Offset(offset).Limit(limit).Order("sm.id DESC").Scan(&raws).Error
	if err != nil {
		return nil, 0, err
	}

	movements := make([]stock.StockMovement, len(raws))
	for i, row := range raws {
		lotNum := ""
		if row.LotNumber != nil {
			lotNum = *row.LotNumber
		}
		suppLot := ""
		if row.SupplierLot != nil {
			suppLot = *row.SupplierLot
		}
		expDate := ""
		if row.ExpiryDate != nil {
			expDate = *row.ExpiryDate
		}

		// Fallback: extract lot from Note if not linked to stock_lot_id directly (e.g. "(Lot: LOT-xxx)")
		if lotNum == "" && strings.Contains(row.Note, "(Lot:") {
			parts := strings.Split(row.Note, "(Lot:")
			if len(parts) > 1 {
				extracted := strings.TrimRight(strings.TrimSpace(parts[1]), ")")
				lotNum = extracted
			}
		}

		movements[i] = stock.StockMovement{
			ID:                row.ID,
			SKUID:             row.SKUID,
			SKUCode:           row.SKUCode,
			WarehouseID:       row.WarehouseID,
			StockLotID:        row.StockLotID,
			SourceFormulaCode: row.SourceFormulaCode,
			Channel:           row.Channel,
			Type:              row.Type,
			Quantity:          row.Quantity,
			BeforeQty:         row.BeforeQty,
			AfterQty:          row.AfterQty,
			ReferenceType:     row.ReferenceType,
			ReferenceID:       row.ReferenceID,
			Note:              row.Note,
			CreatedAt:         row.CreatedAt,
			LotNumber:         lotNum,
			SupplierLot:       suppLot,
			ExpiryDate:        expDate,
		}
	}

	return movements, total, nil
}

func (r *StockRepository) CreateLot(ctx context.Context, lot *stock.StockLot) error {
	if lot.AvailableQty == 0 && lot.Quantity > 0 {
		lot.AvailableQty = lot.Quantity - lot.ReservedQty
	}
	return r.getDB(ctx).Create(lot).Error
}

// GetAvailableLotsForUpdate loads lots with available stock ordered by FEFO (expiry_date ASC, id ASC)
// and locks rows with FOR UPDATE to prevent race conditions during allocation.
func (r *StockRepository) GetAvailableLotsForUpdate(ctx context.Context, skuID, warehouseID uint) ([]stock.StockLot, error) {
	var lots []stock.StockLot
	err := r.getDB(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("sku_id = ? AND warehouse_id = ? AND (quantity - reserved_qty) > 0", skuID, warehouseID).
		Order("expiry_date ASC, id ASC").
		Find(&lots).Error
	if err != nil {
		return nil, err
	}
	return lots, nil
}

func (r *StockRepository) DeductLotQuantity(ctx context.Context, lotID uint, qty int) (*stock.StockLot, error) {
	var item stock.StockLot
	db := r.getDB(ctx)
	deductFn := func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", lotID).
			First(&item).Error
		if err != nil {
			return err
		}
		item.Quantity -= qty
		if item.Quantity < 0 {
			item.Quantity = 0
		}
		item.AvailableQty = item.Quantity - item.ReservedQty
		return tx.Save(&item).Error
	}
	if err := db.Transaction(deductFn); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *StockRepository) FindLotsBySKU(ctx context.Context, skuID, warehouseID uint) ([]stock.StockLot, error) {
	var lots []stock.StockLot
	tx := r.getDB(ctx).Where("sku_id = ?", skuID)
	if warehouseID != 0 {
		tx = tx.Where("warehouse_id = ?", warehouseID)
	}
	err := tx.Order("expiry_date ASC, id ASC").Find(&lots).Error
	if err != nil {
		return nil, err
	}
	return lots, nil
}

