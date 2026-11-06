package postgres

import (
	"context"
	"fmt"
	"strings"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainCatalog "chawy-erp-api/internal/domain/catalog"
	domainFormula "chawy-erp-api/internal/domain/formula"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	pkgDatabase "chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CatalogRepository implements domainCatalog.Repository. Every method picks up
// the transaction from the context first so usecases can wrap multi-step flows
// (create, rename cascade, delete, import rows) in a single database tx.
type CatalogRepository struct {
	db *gorm.DB
}

func NewCatalogRepository(db *gorm.DB) domainCatalog.Repository {
	return &CatalogRepository{db: db}
}

func (r *CatalogRepository) exec(ctx context.Context) *gorm.DB {
	return pkgDatabase.GetDBFromContext(ctx, r.db)
}

// ---- reads: product ----

func (r *CatalogRepository) Count(ctx context.Context, q domainCatalog.ProductQuery) (int64, error) {
	var total int64
	query := r.productListQuery(ctx, q)
	return total, query.Count(&total).Error
}

func (r *CatalogRepository) Search(ctx context.Context, q domainCatalog.ProductQuery) ([]domainSKU.SKU, error) {
	var skus []domainSKU.SKU
	query := r.productListQuery(ctx, q)
	if err := query.Offset((q.Page - 1) * q.Limit).Limit(q.Limit).Order("id DESC").Find(&skus).Error; err != nil {
		return nil, err
	}
	return skus, nil
}

// productListQuery replicates the original workspace GetProducts filters.
func (r *CatalogRepository) productListQuery(ctx context.Context, q domainCatalog.ProductQuery) *gorm.DB {
	db := pkgDatabase.GetDBFromContext(ctx, r.db).Model(&domainSKU.SKU{})
	if q.Search != "" {
		s := "%" + q.Search + "%"
		db = db.Where("sku ILIKE ? OR name ILIKE ? OR barcode ILIKE ?", s, s, s)
	}
	// Filter by type / category (e.g. 'Finished Product', 'Raw Material', 'Bundle')
	if q.ProdType != "" && !strings.EqualFold(q.ProdType, "all") {
		if strings.EqualFold(q.ProdType, "bundle") {
			db = db.Where("is_bundle = true")
		} else {
			db = db.Where("(category ILIKE ? OR (? = 'Finished Product' AND (category IS NULL OR category = '')))", q.ProdType, q.ProdType)
		}
	}
	// Filter by isBundle explicitly (F2)
	if q.IsBundle != "" && !strings.EqualFold(q.IsBundle, "all") {
		if strings.EqualFold(q.IsBundle, "true") {
			db = db.Where("is_bundle = true")
		} else if strings.EqualFold(q.IsBundle, "false") {
			db = db.Where("is_bundle = false")
		}
	}
	// Filter by isActive / status
	if q.IsActive != "" && !strings.EqualFold(q.IsActive, "all") {
		if strings.EqualFold(q.IsActive, "true") || strings.EqualFold(q.IsActive, "active") {
			db = db.Where("status = 'active'")
		} else if strings.EqualFold(q.IsActive, "false") || strings.EqualFold(q.IsActive, "inactive") {
			db = db.Where("status != 'active'")
		}
	}
	return db
}

func (r *CatalogRepository) FindBySKUExact(ctx context.Context, code string) (*domainSKU.SKU, error) {
	var s domainSKU.SKU
	if err := r.exec(ctx).Where("sku = ?", code).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *CatalogRepository) FindByUpperSKU(ctx context.Context, code string) (*domainSKU.SKU, error) {
	var s domainSKU.SKU
	if err := r.exec(ctx).Where("UPPER(sku) = ?", strings.ToUpper(strings.TrimSpace(code))).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *CatalogRepository) FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	var s domainSKU.SKU
	if err := r.exec(ctx).First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *CatalogRepository) ExistsUpperExcluding(ctx context.Context, code string, id uint) (bool, error) {
	var dup domainSKU.SKU
	err := r.exec(ctx).Where("UPPER(sku) = ? AND id != ?", code, id).First(&dup).Error
	// The original handler treated any lookup error as "still unique" and
	// continued; only a successful fetch meant the code was taken.
	return err == nil, nil
}

func (r *CatalogRepository) ExistsUpper(ctx context.Context, code string) (bool, error) {
	var s domainSKU.SKU
	err := r.exec(ctx).Where("UPPER(sku) = ?", code).First(&s).Error
	return err == nil, nil
}

func (r *CatalogRepository) StockBySKUID(ctx context.Context, skuID uint) (*domainStock.Stock, error) {
	var stk domainStock.Stock
	_ = r.exec(ctx).Where("sku_id = ?", skuID).First(&stk).Error
	return &stk, nil
}

// ---- reads: list aggregates ----

func (r *CatalogRepository) SumStocks(ctx context.Context, skuIDs []uint) ([]domainCatalog.StockSumRow, error) {
	var sums []domainCatalog.StockSumRow
	if err := r.exec(ctx).Model(&domainStock.Stock{}).
		Select("sku_id, COALESCE(SUM(quantity), 0) AS quantity, COALESCE(SUM(available_qty), 0) AS available_qty, COALESCE(SUM(reserved_qty), 0) AS reserved_qty").
		Where("sku_id IN ?", skuIDs).
		Group("sku_id").
		Scan(&sums).Error; err != nil {
		return nil, err
	}
	return sums, nil
}

func (r *CatalogRepository) SumUsed(ctx context.Context, skuIDs []uint, codes []string) ([]domainCatalog.UsedSumRow, error) {
	var sums []domainCatalog.UsedSumRow
	_ = r.exec(ctx).Model(&domainStock.StockMovement{}).
		Select("COALESCE(sku_id, 0) AS sku_id, UPPER(TRIM(sku_code)) AS sku_code, COALESCE(SUM(quantity), 0) AS qty").
		Where("(sku_id IN ? OR UPPER(TRIM(sku_code)) IN ?) AND type = ?", skuIDs, codes, domainStock.MovementOut).
		Group("sku_id, UPPER(TRIM(sku_code))").
		Scan(&sums).Error
	return sums, nil
}

func (r *CatalogRepository) TikTokMappings(ctx context.Context, erpCodes []string) ([]domainTikTok.SKUMapping, error) {
	var mappings []domainTikTok.SKUMapping
	if err := r.exec(ctx).
		Where("UPPER(TRIM(erp_sku)) IN ?", erpCodes).
		Find(&mappings).Error; err != nil {
		return nil, err
	}
	return mappings, nil
}

func (r *CatalogRepository) TikTokMappingsFor(ctx context.Context, erpCode string) ([]domainTikTok.SKUMapping, error) {
	var mappings []domainTikTok.SKUMapping
	_ = r.exec(ctx).Where("UPPER(TRIM(erp_sku)) = ?", erpCode).Find(&mappings).Error
	return mappings, nil
}

func (r *CatalogRepository) TikTokPending(ctx context.Context, codes []string) ([]domainCatalog.PendingRow, error) {
	var pendingItems []domainCatalog.PendingRow
	if err := r.exec(ctx).Model(&domainTikTok.TiktokOrderItem{}).
		Select("UPPER(TRIM(tiktok_order_items.sku)) AS sku, COALESCE(SUM(tiktok_order_items.qty), 0) AS qty").
		Joins("JOIN tiktok_orders ON tiktok_orders.id = tiktok_order_items.order_id").
		Where("UPPER(TRIM(tiktok_order_items.sku)) IN ? AND tiktok_orders.stock_deducted = false AND UPPER(tiktok_orders.status) NOT IN ('CANCELLED', 'CANCELED', 'COMPLETED', 'DELIVERED', 'SHIPPED')", codes).
		Group("UPPER(TRIM(tiktok_order_items.sku))").
		Scan(&pendingItems).Error; err != nil {
		return nil, err
	}
	return pendingItems, nil
}

func (r *CatalogRepository) TikTokPendingFor(ctx context.Context, codes []string) (int64, error) {
	var reserved int64
	_ = r.exec(ctx).Model(&domainTikTok.TiktokOrderItem{}).
		Joins("JOIN tiktok_orders ON tiktok_orders.id = tiktok_order_items.order_id").
		Where("UPPER(TRIM(tiktok_order_items.sku)) IN ? AND tiktok_orders.stock_deducted = false AND UPPER(tiktok_orders.status) NOT IN ('CANCELLED', 'CANCELED', 'COMPLETED', 'DELIVERED', 'SHIPPED')", codes).
		Select("COALESCE(SUM(tiktok_order_items.qty), 0)").
		Scan(&reserved).Error
	return reserved, nil
}

func (r *CatalogRepository) BundleItems(ctx context.Context, bundleSKUs []string) ([]domainBundle.BundleItem, error) {
	var items []domainBundle.BundleItem
	_ = r.exec(ctx).Where("bundle_sku IN ?", bundleSKUs).Find(&items).Error
	return items, nil
}

func (r *CatalogRepository) BundleItemsFor(ctx context.Context, bundleSKU string) ([]domainBundle.BundleItem, error) {
	var items []domainBundle.BundleItem
	_ = r.exec(ctx).Where("bundle_sku = ?", bundleSKU).Find(&items).Error
	return items, nil
}

func (r *CatalogRepository) ComponentAvailability(ctx context.Context, codes []string) ([]domainCatalog.StockCodeRow, error) {
	var rows []domainCatalog.StockCodeRow
	_ = r.exec(ctx).Model(&domainStock.Stock{}).
		Select("UPPER(TRIM(sku_code)) AS sku_code, COALESCE(SUM(available_qty), 0) AS available_qty").
		Where("UPPER(TRIM(sku_code)) IN ?", codes).
		Group("UPPER(TRIM(sku_code))").
		Scan(&rows).Error
	return rows, nil
}

func (r *CatalogRepository) ComponentAvailabilityFor(ctx context.Context, code string) (int, error) {
	var row struct {
		AvailableQty int `gorm:"column:available_qty"`
	}
	_ = r.exec(ctx).Model(&domainStock.Stock{}).
		Select("COALESCE(SUM(available_qty), 0) AS available_qty").
		Where("UPPER(TRIM(sku_code)) = ?", code).
		Scan(&row).Error
	return row.AvailableQty, nil
}

func (r *CatalogRepository) AccessoriesForAll(ctx context.Context, skus []string) ([]domainSKU.SKUAccessory, error) {
	var items []domainSKU.SKUAccessory
	_ = r.exec(ctx).Where("sku IN ?", skus).Find(&items).Error
	return items, nil
}

func (r *CatalogRepository) AccessoriesForSKU(ctx context.Context, sku string) ([]domainSKU.SKUAccessory, error) {
	var items []domainSKU.SKUAccessory
	_ = r.exec(ctx).Where("sku = ?", sku).Find(&items).Error
	return items, nil
}

func (r *CatalogRepository) AccessoriesByUpperSKU(ctx context.Context, sku string) ([]domainSKU.SKUAccessory, error) {
	var items []domainSKU.SKUAccessory
	query := r.exec(ctx).Model(&domainSKU.SKUAccessory{})
	if sku != "" {
		query = query.Where("UPPER(sku) = ?", strings.ToUpper(strings.TrimSpace(sku)))
	}
	err := query.Find(&items).Error
	return items, err
}

func (r *CatalogRepository) UsedQtyFor(ctx context.Context, skuID uint, skuUpper string) (int64, error) {
	var usedQty int64
	_ = r.exec(ctx).Model(&domainStock.StockMovement{}).
		Where("(sku_id = ? OR UPPER(TRIM(sku_code)) = ?) AND type = ?", skuID, skuUpper, domainStock.MovementOut).
		Select("COALESCE(SUM(quantity), 0)").
		Scan(&usedQty).Error
	return usedQty, nil
}

// ---- reads: resolve & validation ----

func (r *CatalogRepository) ResolveUpperTrim(ctx context.Context, codes []string) ([]domainSKU.SKU, error) {
	var skus []domainSKU.SKU
	if err := r.exec(ctx).
		Where("UPPER(TRIM(sku)) IN ?", codes).
		Find(&skus).Error; err != nil {
		return nil, err
	}
	return skus, nil
}

func (r *CatalogRepository) FindActiveFormulas(ctx context.Context, codes []string) ([]domainFormula.InventoryFormula, error) {
	var formulas []domainFormula.InventoryFormula
	// Mirrors the original swallow-the-error behaviour: formula lookup failure
	// simply skips the formula fallback.
	if err := r.exec(ctx).
		Where("UPPER(TRIM(code)) IN ? AND is_active = ?", codes, true).
		Find(&formulas).Error; err != nil {
		return nil, nil
	}
	return formulas, nil
}

func (r *CatalogRepository) AccessoryExists(ctx context.Context, accessorySKU string) (bool, error) {
	var s domainSKU.SKU
	err := r.exec(ctx).Where("UPPER(sku) = ?", accessorySKU).First(&s).Error
	return err == nil, nil
}

func (r *CatalogRepository) AccessoryCycleExists(ctx context.Context, accessorySKU, sku string) (bool, error) {
	var cycleCount int64
	_ = r.exec(ctx).Model(&domainSKU.SKUAccessory{}).
		Where("UPPER(sku) = ? AND UPPER(accessory_sku) = ?", accessorySKU, sku).
		Count(&cycleCount).Error
	return cycleCount > 0, nil
}

// ---- writes ----

func (r *CatalogRepository) CreateSKU(ctx context.Context, sku *domainSKU.SKU) error {
	return r.exec(ctx).Create(sku).Error
}

func (r *CatalogRepository) SaveSKU(ctx context.Context, sku *domainSKU.SKU) error {
	return r.exec(ctx).Save(sku).Error
}

func (r *CatalogRepository) DeleteSKU(ctx context.Context, id uint) error {
	return r.exec(ctx).Delete(&domainSKU.SKU{}, id).Error
}

func (r *CatalogRepository) LockSKU(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	var locked domainSKU.SKU
	if err := r.exec(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, id).Error; err != nil {
		return nil, err
	}
	return &locked, nil
}

func (r *CatalogRepository) CreateStock(ctx context.Context, stock *domainStock.Stock) error {
	return r.exec(ctx).Create(stock).Error
}

func (r *CatalogRepository) DeleteStocksBySKUID(ctx context.Context, skuID uint) error {
	return r.exec(ctx).Where("sku_id = ?", skuID).Delete(&domainStock.Stock{}).Error
}

func (r *CatalogRepository) CreateMovement(ctx context.Context, movement *domainStock.StockMovement) error {
	return r.exec(ctx).Create(movement).Error
}

func (r *CatalogRepository) CreateBundleItem(ctx context.Context, item *domainBundle.BundleItem) error {
	return r.exec(ctx).Create(item).Error
}

func (r *CatalogRepository) DeleteBundleItemsByBundleSKU(ctx context.Context, bundleSKU string) error {
	return r.exec(ctx).Where("bundle_sku = ?", bundleSKU).Delete(&domainBundle.BundleItem{}).Error
}

func (r *CatalogRepository) CreateAccessory(ctx context.Context, item *domainSKU.SKUAccessory) error {
	return r.exec(ctx).Create(item).Error
}

func (r *CatalogRepository) DeleteAccessoriesBySKU(ctx context.Context, sku string) error {
	return r.exec(ctx).Where("sku = ?", sku).Delete(&domainSKU.SKUAccessory{}).Error
}

// CountReferences runs the FULL-13 reference check inside the caller's tx so
// the check and the delete share one transaction and one row lock.
func (r *CatalogRepository) CountReferences(ctx context.Context, sku *domainSKU.SKU) (*domainCatalog.ReferenceCounts, error) {
	db := pkgDatabase.GetDBFromContext(ctx, r.db)
	var refs domainCatalog.ReferenceCounts
	// For bundle products, stock and movements are virtual (derived from child
	// components), so only orders, POs, quotations and other bundle/accessory
	// parents are checked.
	if sku.IsBundle {
		if err := db.Raw(`
			SELECT
				0 AS positive_stock,
				0 AS movements,
				(SELECT COUNT(*) FROM order_items WHERE sku = ?) AS order_items,
				(SELECT COUNT(*) FROM po_items WHERE sku = ?) AS po_items,
				(SELECT COUNT(*) FROM quotation_lines WHERE sku = ?) AS quotation_lines,
				(SELECT COUNT(*) FROM bundle_items WHERE component_sku = ?) AS used_as_comp,
				(SELECT COUNT(*) FROM sku_accessories WHERE accessory_sku = ?) AS used_as_acc
		`, sku.SKU, sku.SKU, sku.SKU, sku.SKU, sku.SKU).Scan(&refs).Error; err != nil {
			return nil, fmt.Errorf("reference check failed: %w", err)
		}
		return &refs, nil
	}
	if err := db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM stocks WHERE sku_id = ? AND (quantity > 0 OR available_qty > 0)) AS positive_stock,
			(SELECT COUNT(*) FROM stock_movements WHERE sku_id = ?) AS movements,
			(SELECT COUNT(*) FROM order_items WHERE sku = ?) AS order_items,
			(SELECT COUNT(*) FROM po_items WHERE sku = ?) AS po_items,
			(SELECT COUNT(*) FROM quotation_lines WHERE sku = ?) AS quotation_lines,
			(SELECT COUNT(*) FROM bundle_items WHERE component_sku = ?) AS used_as_comp,
			(SELECT COUNT(*) FROM sku_accessories WHERE accessory_sku = ?) AS used_as_acc
	`, sku.ID, sku.ID, sku.SKU, sku.SKU, sku.SKU, sku.SKU, sku.SKU).Scan(&refs).Error; err != nil {
		return nil, fmt.Errorf("reference check failed: %w", err)
	}
	return &refs, nil
}

// CascadeSKURename updates all tables that reference the old SKU string to use
// the new SKU string. Must run inside the caller's transaction. Tables that
// reference by sku_id (numeric FK) need no update: the record ID stays the same.
func (r *CatalogRepository) CascadeSKURename(ctx context.Context, oldSKU, newSKU string) error {
	db := pkgDatabase.GetDBFromContext(ctx, r.db)
	// bundle_items: both bundle_sku and component_sku columns
	if err := db.Exec("UPDATE bundle_items SET bundle_sku = ? WHERE bundle_sku = ?", newSKU, oldSKU).Error; err != nil {
		return fmt.Errorf("cascade bundle_items.bundle_sku: %w", err)
	}
	if err := db.Exec("UPDATE bundle_items SET component_sku = ? WHERE component_sku = ?", newSKU, oldSKU).Error; err != nil {
		return fmt.Errorf("cascade bundle_items.component_sku: %w", err)
	}

	// sku_accessories: both sku and accessory_sku columns
	if err := db.Exec("UPDATE sku_accessories SET sku = ? WHERE sku = ?", newSKU, oldSKU).Error; err != nil {
		return fmt.Errorf("cascade sku_accessories.sku: %w", err)
	}
	if err := db.Exec("UPDATE sku_accessories SET accessory_sku = ? WHERE accessory_sku = ?", newSKU, oldSKU).Error; err != nil {
		return fmt.Errorf("cascade sku_accessories.accessory_sku: %w", err)
	}

	// order_items
	if err := db.Exec("UPDATE order_items SET sku = ? WHERE sku = ?", newSKU, oldSKU).Error; err != nil {
		return fmt.Errorf("cascade order_items.sku: %w", err)
	}

	// po_items
	if err := db.Exec("UPDATE po_items SET sku = ? WHERE sku = ?", newSKU, oldSKU).Error; err != nil {
		return fmt.Errorf("cascade po_items.sku: %w", err)
	}

	// quotation_lines
	if err := db.Exec("UPDATE quotation_lines SET sku = ? WHERE sku = ?", newSKU, oldSKU).Error; err != nil {
		return fmt.Errorf("cascade quotation_lines.sku: %w", err)
	}

	return nil
}
