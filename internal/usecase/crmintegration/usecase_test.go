package crmintegration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domainCRMIntegration "chawy-erp-api/internal/domain/crmintegration"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/require"
)

// ---- fakes (only the methods the usecase calls are implemented) ----

type fakeSKUs struct {
	domainSKU.Repository
	items []domainSKU.SKU
	query domainSKU.Query
}

func (f *fakeSKUs) FindByID(_ context.Context, id uint) (*domainSKU.SKU, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			cp := f.items[i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeSKUs) FindAll(_ context.Context, q domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	f.query = q
	var out []domainSKU.SKU
	for _, s := range f.items {
		if q.Status != "" && s.Status != q.Status {
			continue
		}
		if q.Search != "" && !strings.Contains(strings.ToLower(s.SKU+s.Name), strings.ToLower(q.Search)) {
			continue
		}
		out = append(out, s)
	}
	return out, int64(len(out)), nil
}

type stockKey struct{ sku, wh uint }

type fakeStock struct {
	domainStock.Repository
	rows      map[stockKey]*domainStock.Stock
	movements []domainStock.StockMovement
	locked    []uint
}

func (f *fakeStock) GetBySKUID(_ context.Context, sku, wh uint) (*domainStock.Stock, error) {
	if s, ok := f.rows[stockKey{sku, wh}]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeStock) GetBySKUIDForUpdate(ctx context.Context, sku, wh uint) (*domainStock.Stock, error) {
	f.locked = append(f.locked, sku)
	return f.GetBySKUID(ctx, sku, wh)
}

func (f *fakeStock) UpdateQuantity(_ context.Context, sku, wh uint, delta int) (*domainStock.Stock, error) {
	s := f.rows[stockKey{sku, wh}]
	s.Quantity += delta
	s.AvailableQty = s.Quantity - s.ReservedQty
	cp := *s
	return &cp, nil
}

func (f *fakeStock) CreateMovement(_ context.Context, m *domainStock.StockMovement) error {
	f.movements = append(f.movements, *m)
	return nil
}

type fakeIssues struct {
	rows []domainCRMIntegration.GoodsIssue
}

func (f *fakeIssues) FindByKey(_ context.Context, key string) (*domainCRMIntegration.GoodsIssue, error) {
	for i := range f.rows {
		if f.rows[i].IdempotencyKey == key {
			cp := f.rows[i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeIssues) Create(_ context.Context, gi *domainCRMIntegration.GoodsIssue) error {
	gi.ID = uint(len(f.rows) + 1)
	f.rows = append(f.rows, *gi)
	return nil
}

func (f *fakeIssues) SetRef(_ context.Context, id uint, ref string) error {
	f.rows[id-1].GIRef = ref
	return nil
}

type fakeTx struct{ calls int }

func (f *fakeTx) Transaction(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

type env struct {
	uc     *usecase
	skus   *fakeSKUs
	stock  *fakeStock
	issues *fakeIssues
	tx     *fakeTx
}

func newEnv() *env {
	skus := &fakeSKUs{items: []domainSKU.SKU{
		{ID: 1, SKU: "CHK-01", Name: "ขนมไก่", Status: "active", Image: "https://img/1.jpg", Price: 99},
		{ID: 2, SKU: "TUN-01", Name: "ขนมทูน่า", Status: "active"},
		{ID: 3, SKU: "OLD-01", Name: "เลิกขาย", Status: "inactive"},
	}}
	stock := &fakeStock{rows: map[stockKey]*domainStock.Stock{
		{1, 1}: {SKUID: 1, WarehouseID: 1, Quantity: 10, ReservedQty: 2, AvailableQty: 8},
		{2, 1}: {SKUID: 2, WarehouseID: 1, Quantity: 3, AvailableQty: 3},
		{1, 2}: {SKUID: 1, WarehouseID: 2, Quantity: 50, AvailableQty: 50},
	}}
	issues := &fakeIssues{}
	tx := &fakeTx{}
	uc := New(skus, stock, issues, tx).(*usecase)
	uc.now = func() time.Time { return time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC) }
	return &env{uc: uc, skus: skus, stock: stock, issues: issues, tx: tx}
}

func appCode(t *testing.T, err error) string {
	t.Helper()
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr), "want AppError, got %v", err)
	return appErr.Code
}

func TestSearchSKUsActiveOnlyWithStock(t *testing.T) {
	e := newEnv()
	items, total, err := e.uc.SearchSKUs(context.Background(), " ขนม ", 0, 0, 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Equal(t, "active", e.skus.query.Status)
	require.Equal(t, "ขนม", e.skus.query.Search)
	require.Equal(t, 20, e.skus.query.Limit)
	require.Equal(t, 8, items[0].AvailableQty, "available excludes reserved")
	require.Equal(t, "https://img/1.jpg", items[0].ImageURL)

	items, _, err = e.uc.SearchSKUs(context.Background(), "CHK", 2, 1, 10)
	require.NoError(t, err)
	require.Equal(t, 50, items[0].AvailableQty, "other warehouse")
}

func TestStockLimitsAndDefaults(t *testing.T) {
	e := newEnv()
	items, err := e.uc.Stock(context.Background(), []uint{1, 2, 1, 99}, 0)
	require.NoError(t, err)
	require.Len(t, items, 3, "duplicates are dropped")
	require.Equal(t, 8, items[0].AvailableQty)
	require.Equal(t, 0, items[2].AvailableQty, "unknown SKU reports zero")
	require.EqualValues(t, 1, items[0].WarehouseID)

	_, err = e.uc.Stock(context.Background(), nil, 0)
	require.Equal(t, "VALIDATION_ERROR", appCode(t, err))
	ids := make([]uint, domainCRMIntegration.MaxStockSKUs+1)
	for i := range ids {
		ids[i] = uint(i + 1)
	}
	_, err = e.uc.Stock(context.Background(), ids, 0)
	require.Equal(t, "VALIDATION_ERROR", appCode(t, err))
}

func TestGoodsIssueDeductsAndIsIdempotent(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	in := GoodsIssueInput{
		IdempotencyKey: "RD2611-000001", Reference: "RD2611-000001",
		Items: []GoodsIssueLine{{SKUID: 1, Qty: 2}, {SKUID: 2, Qty: 1}, {SKUID: 1, Qty: 1}},
	}
	res, err := e.uc.CreateGoodsIssue(ctx, in)
	require.NoError(t, err)
	require.Equal(t, "CRMGI-000001", res.GIRef)
	require.False(t, res.Replayed)
	require.Len(t, res.Items, 2, "duplicate lines are merged")
	require.Equal(t, 7, e.stock.rows[stockKey{1, 1}].Quantity)
	require.Equal(t, 5, e.stock.rows[stockKey{1, 1}].AvailableQty)
	require.Equal(t, 2, e.stock.rows[stockKey{2, 1}].Quantity)
	require.Len(t, e.stock.movements, 2)
	m := e.stock.movements[0]
	require.Equal(t, domainStock.MovementOut, m.Type)
	require.Equal(t, "CRM_REWARD", m.ReferenceType)
	require.Equal(t, "CRMGI-000001", m.ReferenceID)
	require.Equal(t, 10, m.BeforeQty)
	require.Equal(t, 7, m.AfterQty)

	// A retry with the same key deducts nothing and returns the same ref.
	again, err := e.uc.CreateGoodsIssue(ctx, in)
	require.NoError(t, err)
	require.True(t, again.Replayed)
	require.Equal(t, "CRMGI-000001", again.GIRef)
	require.Equal(t, 7, e.stock.rows[stockKey{1, 1}].Quantity)
	require.Len(t, e.stock.movements, 2)
	require.Len(t, e.issues.rows, 1)
}

func TestGoodsIssueShortageDeductsNothing(t *testing.T) {
	e := newEnv()
	_, err := e.uc.CreateGoodsIssue(context.Background(), GoodsIssueInput{
		IdempotencyKey: "RD-2",
		Items:          []GoodsIssueLine{{SKUID: 1, Qty: 9}, {SKUID: 2, Qty: 1}},
	})
	var short *InsufficientStockError
	require.True(t, errors.As(err, &short))
	require.True(t, errors.Is(err, appErrors.ErrInsufficientStock))
	require.Len(t, short.Shortages, 1)
	require.Equal(t, Shortage{SKUID: 1, SKUCode: "CHK-01", Requested: 9, Available: 8}, short.Shortages[0],
		"reserved stock is not available for rewards")
	require.Equal(t, 10, e.stock.rows[stockKey{1, 1}].Quantity)
	require.Equal(t, 3, e.stock.rows[stockKey{2, 1}].Quantity)
	require.Empty(t, e.stock.movements)
	require.Empty(t, e.issues.rows, "the key stays free so the CRM can retry later")
}

func TestGoodsIssueValidation(t *testing.T) {
	e := newEnv()
	ctx := context.Background()
	cases := map[string]GoodsIssueInput{
		"no key":      {Items: []GoodsIssueLine{{SKUID: 1, Qty: 1}}},
		"long key":    {IdempotencyKey: strings.Repeat("k", 101), Items: []GoodsIssueLine{{SKUID: 1, Qty: 1}}},
		"no items":    {IdempotencyKey: "k"},
		"zero qty":    {IdempotencyKey: "k", Items: []GoodsIssueLine{{SKUID: 1, Qty: 0}}},
		"missing sku": {IdempotencyKey: "k", Items: []GoodsIssueLine{{SKUID: 0, Qty: 1}}},
	}
	for name, in := range cases {
		_, err := e.uc.CreateGoodsIssue(ctx, in)
		require.Equal(t, "VALIDATION_ERROR", appCode(t, err), name)
	}
	_, err := e.uc.CreateGoodsIssue(ctx, GoodsIssueInput{IdempotencyKey: "k", Items: []GoodsIssueLine{{SKUID: 42, Qty: 1}}})
	require.Equal(t, "SKU_NOT_FOUND", appCode(t, err))
	require.Zero(t, e.tx.calls-1, "validation errors never open a transaction")
}
