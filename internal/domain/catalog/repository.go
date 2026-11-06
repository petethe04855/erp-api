// Package catalog holds the repository contract for the workspace product
// catalog: list, detail, resolve, accessories, import and SKU rename cascade.
// Writes must run through the repository so every database concern stays out
// of the delivery layer; the usecase coordinates transactions via TxManager
// and the implementation JOINs the caller's transaction from the context.
package catalog

import (
	"context"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainFormula "chawy-erp-api/internal/domain/formula"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
)

// ProductQuery carries the workspace product-list filters exactly as the
// workspace screen originally sent them (raw string variants preserved).
type ProductQuery struct {
	Page     int
	Limit    int
	Search   string
	ProdType string // type/category filter; "bundle" maps to is_bundle = true
	IsBundle string // explicit isBundle filter ("true"/"false"/"all"/"")
	IsActive string // isActive/status filter ("true"/"active"/"false"/"inactive"/"all"/"")
}

// StockSumRow is one aggregated stock row across all warehouses.
type StockSumRow struct {
	SKUID        uint `gorm:"column:sku_id"`
	Quantity     int  `gorm:"column:quantity"`
	AvailableQty int  `gorm:"column:available_qty"`
	ReservedQty  int  `gorm:"column:reserved_qty"`
}

// UsedSumRow is one aggregated OUT-movement total keyed by sku_id and code.
type UsedSumRow struct {
	SKUID   uint   `gorm:"column:sku_id"`
	SKUCode string `gorm:"column:sku_code"`
	Qty     int    `gorm:"column:qty"`
}

// PendingRow is one aggregated TikTok pending reserved quantity keyed by SKU.
type PendingRow struct {
	SKU string `gorm:"column:sku"`
	Qty int64  `gorm:"column:qty"`
}

// StockCodeRow is an availability aggregate keyed by upper SKU code.
type StockCodeRow struct {
	SKUCode      string `gorm:"column:sku_code"`
	AvailableQty int    `gorm:"column:available_qty"`
}

// ReferenceCounts mirrors the FULL-13 singular-row reference check.
type ReferenceCounts struct {
	PositiveStock int64 `gorm:"column:positive_stock"`
	Movements     int64 `gorm:"column:movements"`
	OrderItems    int64 `gorm:"column:order_items"`
	POItems       int64 `gorm:"column:po_items"`
	QuotationLn   int64 `gorm:"column:quotation_lines"`
	UsedAsComp    int64 `gorm:"column:used_as_comp"`
	UsedAsAcc     int64 `gorm:"column:used_as_acc"`
}

// Repository is the catalog contract used by the workspace endpoints.
type Repository interface {
	// reads — product
	Count(ctx context.Context, q ProductQuery) (int64, error)
	Search(ctx context.Context, q ProductQuery) ([]domainSKU.SKU, error)
	FindBySKUExact(ctx context.Context, code string) (*domainSKU.SKU, error)
	FindByUpperSKU(ctx context.Context, code string) (*domainSKU.SKU, error)
	FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error)
	ExistsUpperExcluding(ctx context.Context, code string, id uint) (bool, error)
	ExistsUpper(ctx context.Context, code string) (bool, error)
	StockBySKUID(ctx context.Context, skuID uint) (*domainStock.Stock, error)

	// reads — list aggregates
	SumStocks(ctx context.Context, skuIDs []uint) ([]StockSumRow, error)
	SumUsed(ctx context.Context, skuIDs []uint, codes []string) ([]UsedSumRow, error)
	TikTokMappings(ctx context.Context, erpCodes []string) ([]domainTikTok.SKUMapping, error)
	TikTokMappingsFor(ctx context.Context, erpCode string) ([]domainTikTok.SKUMapping, error)
	TikTokPending(ctx context.Context, codes []string) ([]PendingRow, error)
	TikTokPendingFor(ctx context.Context, codes []string) (int64, error)
	BundleItems(ctx context.Context, bundleSKUs []string) ([]domainBundle.BundleItem, error)
	BundleItemsFor(ctx context.Context, bundleSKU string) ([]domainBundle.BundleItem, error)
	ComponentAvailability(ctx context.Context, codes []string) ([]StockCodeRow, error)
	ComponentAvailabilityFor(ctx context.Context, code string) (int, error)
	AccessoriesForAll(ctx context.Context, skus []string) ([]domainSKU.SKUAccessory, error)
	AccessoriesForSKU(ctx context.Context, sku string) ([]domainSKU.SKUAccessory, error)
	AccessoriesByUpperSKU(ctx context.Context, sku string) ([]domainSKU.SKUAccessory, error)
	UsedQtyFor(ctx context.Context, skuID uint, skuUpper string) (int64, error)

	// reads — resolve & validation
	ResolveUpperTrim(ctx context.Context, codes []string) ([]domainSKU.SKU, error)
	FindActiveFormulas(ctx context.Context, codes []string) ([]domainFormula.InventoryFormula, error)
	AccessoryExists(ctx context.Context, accessorySKU string) (bool, error)
	AccessoryCycleExists(ctx context.Context, accessorySKU, sku string) (bool, error)

	// writes (participate in the tx carried by the context)
	CreateSKU(ctx context.Context, sku *domainSKU.SKU) error
	SaveSKU(ctx context.Context, sku *domainSKU.SKU) error
	DeleteSKU(ctx context.Context, id uint) error
	LockSKU(ctx context.Context, id uint) (*domainSKU.SKU, error)
	CreateStock(ctx context.Context, stock *domainStock.Stock) error
	DeleteStocksBySKUID(ctx context.Context, skuID uint) error
	CreateMovement(ctx context.Context, movement *domainStock.StockMovement) error
	CreateBundleItem(ctx context.Context, item *domainBundle.BundleItem) error
	DeleteBundleItemsByBundleSKU(ctx context.Context, bundleSKU string) error
	CreateAccessory(ctx context.Context, item *domainSKU.SKUAccessory) error
	DeleteAccessoriesBySKU(ctx context.Context, sku string) error
	CountReferences(ctx context.Context, sku *domainSKU.SKU) (*ReferenceCounts, error)
	CascadeSKURename(ctx context.Context, oldSKU, newSKU string) error
}
