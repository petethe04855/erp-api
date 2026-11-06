package postgres

import (
	"context"
	"errors"

	"chawy-erp-api/internal/domain/purchasing"
	"chawy-erp-api/internal/domain/stock"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// FindPOByRef resolves a PO by document number or numeric id, mirroring the
// lookup the goods-receive screen performs.
func (r *PurchasingRepository) FindPOByRef(ctx context.Context, ref string) (*purchasing.PurchaseOrder, error) {
	var po purchasing.PurchaseOrder
	err := r.getDB(ctx).Preload("Items").Where("po_no = ? OR id::text = ?", ref, ref).First(&po).Error
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

// FindPOByIDForUpdate locks the PO row with SELECT ... FOR UPDATE inside the
// caller's transaction (FULL-09).
func (r *PurchasingRepository) FindPOByIDForUpdate(ctx context.Context, id uint) (*purchasing.PurchaseOrder, error) {
	var po purchasing.PurchaseOrder
	err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").First(&po, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &po, nil
}

func (r *PurchasingRepository) UpdatePOItemReceivedQty(ctx context.Context, poItemID uint, receivedQty int) error {
	return r.getDB(ctx).Model(&purchasing.POItem{}).Where("id = ?", poItemID).
		Update("received_qty", receivedQty).Error
}

// CreateGoodsReceiveDoc persists the GR header and its items in one call on
// the current connection (transaction-aware via context).
func (r *PurchasingRepository) CreateGoodsReceiveDoc(ctx context.Context, gr *purchasing.GoodsReceive, items []purchasing.GoodsReceiveItem) error {
	db := r.getDB(ctx)
	if err := db.Create(gr).Error; err != nil {
		return err
	}
	for i := range items {
		items[i].GRID = gr.ID
		if err := db.Create(&items[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// FindAllGoodsReceives lists goods-receive documents for the workspace screen.
func (r *PurchasingRepository) FindAllGoodsReceives(ctx context.Context, q purchasing.GoodsReceiveQuery) ([]purchasing.GoodsReceive, int64, error) {
	var items []purchasing.GoodsReceive
	var total int64

	tx := r.getDB(ctx).Model(&purchasing.GoodsReceive{})
	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("code ILIKE ? OR po_ref ILIKE ? OR supplier_name ILIKE ?", pat, pat, pat)
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

func (r *PurchasingRepository) FindGoodsReceiveByID(ctx context.Context, id uint) (*purchasing.GoodsReceive, error) {
	var gr purchasing.GoodsReceive
	err := r.getDB(ctx).Preload("Items").First(&gr, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &gr, nil
}

// FindAllGoodsIssues lists OUT stock movements, which back the goods-issue
// history screen.
func (r *PurchasingRepository) FindAllGoodsIssues(ctx context.Context, q purchasing.GoodsIssueQuery) ([]stock.StockMovement, int64, error) {
	var items []stock.StockMovement
	var total int64

	tx := r.getDB(ctx).Model(&stock.StockMovement{}).Where("type = ?", stock.MovementOut)
	if q.Search != "" {
		pat := "%" + q.Search + "%"
		tx = tx.Where("sku_code ILIKE ? OR note ILIKE ? OR reference_id ILIKE ?", pat, pat, pat)
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

func (r *PurchasingRepository) FindGoodsIssueByID(ctx context.Context, id uint) (*stock.StockMovement, error) {
	var m stock.StockMovement
	err := r.getDB(ctx).First(&m, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}
