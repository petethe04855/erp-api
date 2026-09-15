package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"chawy-erp-api/internal/domain/sku"
	pkgDatabase "chawy-erp-api/pkg/database"

	"gorm.io/gorm"
)


type SKURepository struct {
	db *gorm.DB
}

func NewSKURepository(db *gorm.DB) sku.Repository {
	return &SKURepository{db: db}
}

func (r *SKURepository) handle(ctx context.Context) *gorm.DB {
	return pkgDatabase.GetDBFromContext(ctx, r.db)
}

func (r *SKURepository) Create(ctx context.Context, item *sku.SKU) error {
	return r.handle(ctx).Create(item).Error
}

func (r *SKURepository) FindByID(ctx context.Context, id uint) (*sku.SKU, error) {
	var item sku.SKU
	err := r.handle(ctx).First(&item, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *SKURepository) FindBySKU(ctx context.Context, skuCode string) (*sku.SKU, error) {
	var item sku.SKU
	err := r.handle(ctx).Where("sku = ?", skuCode).First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *SKURepository) FindAll(ctx context.Context, q sku.Query) ([]sku.SKU, int64, error) {
	var items []sku.SKU
	var total int64

	tx := r.handle(ctx).Model(&sku.SKU{})

	if q.Search != "" {
		searchPattern := "%" + q.Search + "%"
		tx = tx.Where("sku ILIKE ? OR name ILIKE ? OR barcode ILIKE ?", searchPattern, searchPattern, searchPattern)
	}

	if q.Category != "" {
		tx = tx.Where("category = ?", q.Category)
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
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *SKURepository) Update(ctx context.Context, item *sku.SKU) error {
	return r.handle(ctx).Save(item).Error
}

func (r *SKURepository) Delete(ctx context.Context, id uint) error {
	return r.handle(ctx).Delete(&sku.SKU{}, id).Error
}

func (r *SKURepository) ExistsBySKU(ctx context.Context, skuCode string) (bool, error) {
	var count int64
	err := r.handle(ctx).Model(&sku.SKU{}).Where("sku = ?", skuCode).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SKURepository) GetReceiptStatsBatch(ctx context.Context, skuIDs []uint) (map[uint]sku.SKUBatchReceiptStat, error) {
	result := make(map[uint]sku.SKUBatchReceiptStat, len(skuIDs))
	if len(skuIDs) == 0 {
		return result, nil
	}

	for _, id := range skuIDs {
		result[id] = sku.SKUBatchReceiptStat{SKUID: id}
	}

	db := r.handle(ctx)

	// Subquery/CTE to fetch distinct receipt events per SKU:
	// A receipt event comes from stock_movements where type = 'IN'.
	// In addition, if a Goods Receive exists, its receive_date is the authoritative receipt time.
	type AggRow struct {
		SKUID          uint       `gorm:"column:sku_id"`
		LastReceivedAt *time.Time `gorm:"column:last_received_at"`
		ReceiptCount   int        `gorm:"column:receipt_count"`
	}

	var rows []AggRow
	err := db.Raw(`
		WITH receipt_events AS (
			SELECT 
				sm.sku_id,
				COALESCE(
					gr.receive_date::timestamp with time zone,
					sl.received_at,
					sm.created_at
				) AS event_time
			FROM stock_movements sm
			LEFT JOIN goods_receives gr ON (sm.reference_type = 'GOODS_RECEIVE' AND sm.reference_id = gr.code)
			LEFT JOIN stock_lots sl ON sm.stock_lot_id = sl.id
			WHERE sm.sku_id IN ? AND sm.type = 'IN'
		)
		SELECT 
			sku_id,
			MAX(event_time) AS last_received_at,
			COUNT(*) AS receipt_count
		FROM receipt_events
		GROUP BY sku_id
	`, skuIDs).Scan(&rows).Error

	if err != nil {
		// Fallback for simpler engines or test SQLite where raw cast might vary
		type SimpleAgg struct {
			SKUID          uint       `gorm:"column:sku_id"`
			LastReceivedAt *time.Time `gorm:"column:last_received_at"`
			ReceiptCount   int        `gorm:"column:receipt_count"`
		}
		var simpleRows []SimpleAgg
		fallbackErr := db.Table("stock_movements").
			Select("sku_id, MAX(created_at) as last_received_at, COUNT(*) as receipt_count").
			Where("sku_id IN ? AND type = 'IN'", skuIDs).
			Group("sku_id").
			Scan(&simpleRows).Error
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		for _, sr := range simpleRows {
			result[sr.SKUID] = sku.SKUBatchReceiptStat{
				SKUID:          sr.SKUID,
				LastReceivedAt: sr.LastReceivedAt,
				ReceiptCount:   sr.ReceiptCount,
			}
		}
		return result, nil
	}

	for _, row := range rows {
		result[row.SKUID] = sku.SKUBatchReceiptStat{
			SKUID:          row.SKUID,
			LastReceivedAt: row.LastReceivedAt,
			ReceiptCount:   row.ReceiptCount,
		}
	}

	return result, nil
}

func (r *SKURepository) GetReceiptHistory(ctx context.Context, q sku.SKUReceiptQuery) ([]sku.SKUReceiptItem, int64, error) {
	db := r.handle(ctx)

	// Query StockMovement with type = 'IN' for this SKU, joined with GoodsReceive and StockLot
	type rawReceipt struct {
		ID               uint       `gorm:"column:id"`
		CreatedAt        time.Time  `gorm:"column:created_at"`
		Quantity         int        `gorm:"column:quantity"`
		WarehouseID      uint       `gorm:"column:warehouse_id"`
		ReferenceType    string     `gorm:"column:reference_type"`
		ReferenceID      string     `gorm:"column:reference_id"`
		Note             string     `gorm:"column:note"`
		LotNumber        *string    `gorm:"column:lot_number"`
		SupplierLot      *string    `gorm:"column:supplier_lot"`
		ExpiryDate       *string    `gorm:"column:expiry_date"`
		LotReceivedAt    *time.Time `gorm:"column:lot_received_at"`
		GRReceiveDate    *string    `gorm:"column:gr_receive_date"`
		PurchaseOrderRef *string    `gorm:"column:purchase_order_ref"`
	}

	tx := db.Table("stock_movements sm").
		Select(`
			sm.id,
			sm.created_at,
			sm.quantity,
			sm.warehouse_id,
			sm.reference_type,
			sm.reference_id,
			sm.note,
			sl.lot_number,
			COALESCE(sl.supplier_lot, '') AS supplier_lot,
			COALESCE(sl.expiry_date, '') AS expiry_date,
			sl.received_at AS lot_received_at,
			gr.receive_date AS gr_receive_date,
			gr.po_ref AS purchase_order_ref
		`).
		Joins("LEFT JOIN stock_lots sl ON sm.stock_lot_id = sl.id").
		Joins("LEFT JOIN goods_receives gr ON (sm.reference_type = 'GOODS_RECEIVE' AND sm.reference_id = gr.code)").
		Where("sm.sku_id = ? AND sm.type = 'IN'", q.SKUID)

	if q.WarehouseID != 0 {
		tx = tx.Where("sm.warehouse_id = ?", q.WarehouseID)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (q.Page - 1) * q.Limit
	if offset < 0 {
		offset = 0
	}

	var raws []rawReceipt
	if err := tx.Offset(offset).Limit(q.Limit).Order("sm.created_at DESC, sm.id DESC").Scan(&raws).Error; err != nil {
		return nil, 0, err
	}

	items := make([]sku.SKUReceiptItem, len(raws))
	for i, row := range raws {
		receivedAt := row.CreatedAt

		// Precedence: 1. GoodsReceive.ReceiveDate, 2. StockLot.ReceivedAt, 3. StockMovement.CreatedAt
		if row.GRReceiveDate != nil && *row.GRReceiveDate != "" {
			if parsed, err := time.Parse("2006-01-02", *row.GRReceiveDate); err == nil {
				receivedAt = parsed
			}
		} else if row.LotReceivedAt != nil && !row.LotReceivedAt.IsZero() {
			receivedAt = *row.LotReceivedAt
		}

		sourceType := "STOCK_ADJUSTMENT_IN"
		refUpper := strings.ToUpper(strings.TrimSpace(row.ReferenceType))
		if strings.Contains(refUpper, "GOODS_RECEIVE") || strings.Contains(refUpper, "PO_RECEIVE") || (row.ReferenceID != "" && strings.HasPrefix(strings.ToUpper(row.ReferenceID), "GR-")) {
			sourceType = "GOODS_RECEIVE"
		} else if strings.Contains(refUpper, "INITIAL") || strings.Contains(refUpper, "OPENING") {
			sourceType = "INITIAL_STOCK"
		}

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
		poRef := ""
		if row.PurchaseOrderRef != nil {
			poRef = *row.PurchaseOrderRef
		}

		whName := "คลังหลัก"
		if row.WarehouseID > 1 {
			whName = fmt.Sprintf("Warehouse %d", row.WarehouseID)
		}

		items[i] = sku.SKUReceiptItem{
			ID:               row.ID,
			ReceivedAt:       receivedAt,
			SourceType:       sourceType,
			Quantity:         row.Quantity,
			WarehouseID:      row.WarehouseID,
			WarehouseName:    whName,
			LotNumber:        lotNum,
			SupplierLot:      suppLot,
			ExpiryDate:       expDate,
			ReferenceType:    row.ReferenceType,
			ReferenceID:      row.ReferenceID,
			PurchaseOrderRef: poRef,
			Note:             row.Note,
		}
	}

	return items, total, nil
}

