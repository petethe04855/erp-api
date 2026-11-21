// Package crmintegration serves the CRM backoffice (backoffice-api): SKU
// search for choosing rewards, available stock for the reward stock cache, and
// idempotent goods issues when a reward ships (plan CRM_BACKOFFICE.md §5).
package crmintegration

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainCRMIntegration "chawy-erp-api/internal/domain/crmintegration"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	pkgDatabase "chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"
)

// SKUItem is one SKU with its available quantity in the requested warehouse.
type SKUItem struct {
	ID           uint    `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	ImageURL     string  `json:"imageUrl"`
	Category     string  `json:"category"`
	Price        float64 `json:"price"`
	AvailableQty int     `json:"availableQty"`
}

// StockItem is the available quantity of one SKU.
type StockItem struct {
	SKUID        uint      `json:"skuId"`
	WarehouseID  uint      `json:"warehouseId"`
	AvailableQty int       `json:"availableQty"`
	AsOf         time.Time `json:"asOf"`
}

// GoodsIssueInput is a CRM goods issue request.
type GoodsIssueInput struct {
	IdempotencyKey string
	WarehouseID    uint
	Items          []GoodsIssueLine
	Reference      string
	Note           string
}

// GoodsIssueLine is one requested SKU quantity.
type GoodsIssueLine struct {
	SKUID uint
	Qty   int
}

// GoodsIssueResult is what the CRM stores: the ERP reference of the issue.
type GoodsIssueResult struct {
	GIRef    string                                `json:"giRef"`
	Status   string                                `json:"status"`
	Replayed bool                                  `json:"replayed"`
	Items    []domainCRMIntegration.GoodsIssueItem `json:"items"`
}

// Shortage is one SKU that does not have enough available stock.
type Shortage struct {
	SKUID     uint   `json:"skuId"`
	SKUCode   string `json:"skuCode"`
	Requested int    `json:"requested"`
	Available int    `json:"available"`
}

// InsufficientStockError lists every SKU that is short; nothing was deducted.
type InsufficientStockError struct {
	Shortages []Shortage
}

func (e *InsufficientStockError) Error() string {
	parts := make([]string, 0, len(e.Shortages))
	for _, s := range e.Shortages {
		parts = append(parts, fmt.Sprintf("%s requested %d available %d", s.SKUCode, s.Requested, s.Available))
	}
	return "insufficient stock: " + strings.Join(parts, "; ")
}

func (e *InsufficientStockError) Unwrap() error { return appErrors.ErrInsufficientStock }

// Usecase is the CRM integration.
type Usecase interface {
	SearchSKUs(ctx context.Context, search string, warehouseID uint, page, limit int) ([]SKUItem, int64, error)
	Stock(ctx context.Context, skuIDs []uint, warehouseID uint) ([]StockItem, error)
	CreateGoodsIssue(ctx context.Context, in GoodsIssueInput) (*GoodsIssueResult, error)
}

type usecase struct {
	skus   domainSKU.Repository
	stock  domainStock.Repository
	issues domainCRMIntegration.Repository
	tx     pkgDatabase.TxManager
	now    func() time.Time
}

// New builds the CRM integration usecase.
func New(skus domainSKU.Repository, stock domainStock.Repository, issues domainCRMIntegration.Repository, tx pkgDatabase.TxManager) Usecase {
	return &usecase{skus: skus, stock: stock, issues: issues, tx: tx, now: time.Now}
}

func warehouseOr(id uint) uint {
	if id == 0 {
		return domainCRMIntegration.DefaultWarehouse
	}
	return id
}

func (u *usecase) SearchSKUs(ctx context.Context, search string, warehouseID uint, page, limit int) ([]SKUItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	warehouseID = warehouseOr(warehouseID)
	rows, total, err := u.skus.FindAll(ctx, domainSKU.Query{
		Search: strings.TrimSpace(search), Status: "active", Page: page, Limit: limit,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]SKUItem, 0, len(rows))
	for _, s := range rows {
		available := 0
		stk, err := u.stock.GetBySKUID(ctx, s.ID, warehouseID)
		if err != nil {
			return nil, 0, err
		}
		if stk != nil {
			available = stk.AvailableQty
		}
		out = append(out, SKUItem{
			ID: s.ID, Code: s.SKU, Name: s.Name, ImageURL: s.Image, Category: s.Category,
			Price: s.Price, AvailableQty: available,
		})
	}
	return out, total, nil
}

func (u *usecase) Stock(ctx context.Context, skuIDs []uint, warehouseID uint) ([]StockItem, error) {
	if len(skuIDs) == 0 {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", "skuIds is required", 400)
	}
	if len(skuIDs) > domainCRMIntegration.MaxStockSKUs {
		return nil, appErrors.NewAppError("VALIDATION_ERROR",
			fmt.Sprintf("at most %d skuIds per request", domainCRMIntegration.MaxStockSKUs), 400)
	}
	warehouseID = warehouseOr(warehouseID)
	asOf := u.now()
	seen := make(map[uint]bool, len(skuIDs))
	out := make([]StockItem, 0, len(skuIDs))
	for _, id := range skuIDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		item := StockItem{SKUID: id, WarehouseID: warehouseID, AsOf: asOf}
		stk, err := u.stock.GetBySKUID(ctx, id, warehouseID)
		if err != nil {
			return nil, err
		}
		if stk != nil {
			item.AvailableQty = stk.AvailableQty
		}
		out = append(out, item)
	}
	return out, nil
}

func validateIssue(in GoodsIssueInput) (GoodsIssueInput, error) {
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	in.Reference = strings.TrimSpace(in.Reference)
	in.Note = strings.TrimSpace(in.Note)
	if in.IdempotencyKey == "" || len(in.IdempotencyKey) > domainCRMIntegration.MaxIdempotencyKey {
		return in, appErrors.NewAppError("VALIDATION_ERROR", "idempotencyKey is required (100 characters max)", 400)
	}
	if len(in.Items) == 0 || len(in.Items) > domainCRMIntegration.MaxIssueItems {
		return in, appErrors.NewAppError("VALIDATION_ERROR",
			fmt.Sprintf("items must have 1–%d lines", domainCRMIntegration.MaxIssueItems), 400)
	}
	if len(in.Reference) > 100 || len(in.Note) > 255 {
		return in, appErrors.NewAppError("VALIDATION_ERROR", "reference (100) or note (255) is too long", 400)
	}
	// Merge duplicate SKU lines so each stock row is locked and checked once.
	merged := make([]GoodsIssueLine, 0, len(in.Items))
	index := map[uint]int{}
	for _, it := range in.Items {
		if it.SKUID == 0 || it.Qty <= 0 {
			return in, appErrors.NewAppError("VALIDATION_ERROR", "every item needs skuId and a positive qty", 400)
		}
		if i, ok := index[it.SKUID]; ok {
			merged[i].Qty += it.Qty
			continue
		}
		index[it.SKUID] = len(merged)
		merged = append(merged, it)
	}
	in.Items = merged
	in.WarehouseID = warehouseOr(in.WarehouseID)
	return in, nil
}

func replay(gi *domainCRMIntegration.GoodsIssue) *GoodsIssueResult {
	return &GoodsIssueResult{GIRef: gi.GIRef, Status: "issued", Replayed: true, Items: gi.Items}
}

// CreateGoodsIssue deducts stock for a reward shipment, all lines in one
// transaction, the same way the ERP goods-issue screen does (lock the stock
// row, check total and available quantity, update, write an OUT movement).
// The idempotency key makes retries safe: a key that was already used returns
// the first giRef and deducts nothing. If any line is short, nothing is
// deducted and every short SKU is reported.
func (u *usecase) CreateGoodsIssue(ctx context.Context, in GoodsIssueInput) (*GoodsIssueResult, error) {
	in, err := validateIssue(in)
	if err != nil {
		return nil, err
	}
	if existing, err := u.issues.FindByKey(ctx, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		return replay(existing), nil
	}

	var result *GoodsIssueResult
	err = u.tx.Transaction(ctx, func(txCtx context.Context) error {
		// Re-check inside the transaction; the unique key settles a race.
		if existing, err := u.issues.FindByKey(txCtx, in.IdempotencyKey); err != nil {
			return err
		} else if existing != nil {
			result = replay(existing)
			return nil
		}

		type locked struct {
			line GoodsIssueLine
			sku  *domainSKU.SKU
			stk  *domainStock.Stock
		}
		rows := make([]locked, 0, len(in.Items))
		var shortages []Shortage
		for _, line := range in.Items {
			s, err := u.skus.FindByID(txCtx, line.SKUID)
			if err != nil {
				return err
			}
			if s == nil {
				return appErrors.NewAppError("SKU_NOT_FOUND", fmt.Sprintf("SKU %d not found", line.SKUID), 404)
			}
			stk, err := u.stock.GetBySKUIDForUpdate(txCtx, s.ID, in.WarehouseID)
			if err != nil {
				return err
			}
			available, total := 0, 0
			if stk != nil {
				available, total = stk.AvailableQty, stk.Quantity
			}
			if available < line.Qty || total < line.Qty {
				shortages = append(shortages, Shortage{SKUID: s.ID, SKUCode: s.SKU, Requested: line.Qty, Available: available})
				continue
			}
			rows = append(rows, locked{line: line, sku: s, stk: stk})
		}
		if len(shortages) > 0 {
			return &InsufficientStockError{Shortages: shortages}
		}

		record := &domainCRMIntegration.GoodsIssue{
			IdempotencyKey: in.IdempotencyKey, WarehouseID: in.WarehouseID,
			Reference: in.Reference, Note: in.Note, CreatedAt: u.now(),
		}
		for _, r := range rows {
			record.Items = append(record.Items, domainCRMIntegration.GoodsIssueItem{SKUID: r.sku.ID, SKUCode: r.sku.SKU, Qty: r.line.Qty})
		}
		if err := u.issues.Create(txCtx, record); err != nil {
			return err
		}
		record.GIRef = fmt.Sprintf("CRMGI-%06d", record.ID)
		if err := u.issues.SetRef(txCtx, record.ID, record.GIRef); err != nil {
			return err
		}

		note := in.Note
		if note == "" {
			note = "CRM reward " + in.Reference
		}
		for _, r := range rows {
			updated, err := u.stock.UpdateQuantity(txCtx, r.sku.ID, in.WarehouseID, -r.line.Qty)
			if err != nil {
				return err
			}
			if err := u.stock.CreateMovement(txCtx, &domainStock.StockMovement{
				SKUID: r.sku.ID, SKUCode: r.sku.SKU, WarehouseID: in.WarehouseID, Type: domainStock.MovementOut,
				Quantity: r.line.Qty, BeforeQty: r.stk.Quantity, AfterQty: updated.Quantity,
				Channel: "CRM", ReferenceType: domainCRMIntegration.ReferenceType, ReferenceID: record.GIRef,
				Note: truncate(note, 255), CreatedAt: u.now(),
			}); err != nil {
				return err
			}
		}
		result = &GoodsIssueResult{GIRef: record.GIRef, Status: "issued", Items: record.Items}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
