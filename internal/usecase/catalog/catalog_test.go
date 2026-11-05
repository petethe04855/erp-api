package catalog

import (
	"context"
	"fmt"
	"strings"
	"testing"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainCatalog "chawy-erp-api/internal/domain/catalog"
	domainFormula "chawy-erp-api/internal/domain/formula"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	appErrors "chawy-erp-api/pkg/errors"
	pkgDatabase "chawy-erp-api/pkg/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo embeds the full repository contract so only the exercised methods
// need overriding; calling any other method fails the test loudly (nil map /
// zero behavior) instead of masking a wiring bug.
type fakeRepo struct {
	domainCatalog.Repository

	count  int64
	countE error
	search []domainSKU.SKU
	searchE error

	findBySKU    *domainSKU.SKU
	findBySKUE   error
	findByUpper  *domainSKU.SKU
	findByUpperE error
	findByID     *domainSKU.SKU
	findByIDE    error

	existsUpper        bool
	existsUpperCalls   int
	existsUpperExcl    bool
	stockBySKUID       *domainStock.Stock
	sumStocks          []domainCatalog.StockSumRow
	sumUsed            []domainCatalog.UsedSumRow
	mappings           []domainTikTok.SKUMapping
	mappingsFor        []domainTikTok.SKUMapping
	pending            []domainCatalog.PendingRow
	pendingFor         int64
	bundleItems        []domainBundle.BundleItem
	bundleItemsForSKU  []domainBundle.BundleItem
	compAvail          []domainCatalog.StockCodeRow
	compAvailFor       int
	accessoriesForAll  []domainSKU.SKUAccessory
	accessoriesForSKU  []domainSKU.SKUAccessory
	accessoriesByUpper []domainSKU.SKUAccessory
	usedQtyFor         int64
	resolved           []domainSKU.SKU
	formulas           []domainFormula.InventoryFormula
	accessoryExists    bool
	accessoryCycle     bool

	deletedSKUID uint
	deleteQuiet  bool
	lockSKU      *domainSKU.SKU
	refs         *domainCatalog.ReferenceCounts

	createMovements []domainStock.StockMovement
	createdStocks   []domainStock.Stock
	createdSKUs     []domainSKU.SKU
	createdAccs     []domainSKU.SKUAccessory
	createdBundle   []domainBundle.BundleItem
}

func newTestUsecase(r *fakeRepo, tx pkgDatabase.TxManager) *usecase {
	stats := &fakeStats{stats: map[uint]domainSKU.SKUBatchReceiptStat{}}
	return &usecase{repo: r, stats: stats, txMgr: tx}
}

func (f *fakeRepo) Count(ctx context.Context, q domainCatalog.ProductQuery) (int64, error) {
	return f.count, f.countE
}
func (f *fakeRepo) Search(ctx context.Context, q domainCatalog.ProductQuery) ([]domainSKU.SKU, error) {
	return f.search, f.searchE
}
func (f *fakeRepo) FindBySKUExact(ctx context.Context, code string) (*domainSKU.SKU, error) {
	return f.findBySKU, f.findBySKUE
}
func (f *fakeRepo) FindByUpperSKU(ctx context.Context, code string) (*domainSKU.SKU, error) {
	return f.findByUpper, f.findByUpperE
}
func (f *fakeRepo) FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	return f.findByID, f.findByIDE
}
func (f *fakeRepo) ExistsUpperExcluding(ctx context.Context, code string, id uint) (bool, error) {
	return f.existsUpperExcl, nil
}
func (f *fakeRepo) ExistsUpper(ctx context.Context, code string) (bool, error) {
	f.existsUpperCalls++
	return f.existsUpper, nil
}
func (f *fakeRepo) StockBySKUID(ctx context.Context, skuID uint) (*domainStock.Stock, error) {
	return f.stockBySKUID, nil
}
func (f *fakeRepo) SumStocks(ctx context.Context, skuIDs []uint) ([]domainCatalog.StockSumRow, error) {
	return f.sumStocks, nil
}
func (f *fakeRepo) SumUsed(ctx context.Context, skuIDs []uint, codes []string) ([]domainCatalog.UsedSumRow, error) {
	return f.sumUsed, nil
}
func (f *fakeRepo) TikTokMappings(ctx context.Context, erpCodes []string) ([]domainTikTok.SKUMapping, error) {
	return f.mappings, nil
}
func (f *fakeRepo) TikTokMappingsFor(ctx context.Context, erpCode string) ([]domainTikTok.SKUMapping, error) {
	return f.mappingsFor, nil
}
func (f *fakeRepo) TikTokPending(ctx context.Context, codes []string) ([]domainCatalog.PendingRow, error) {
	return f.pending, nil
}
func (f *fakeRepo) TikTokPendingFor(ctx context.Context, codes []string) (int64, error) {
	return f.pendingFor, nil
}
func (f *fakeRepo) BundleItems(ctx context.Context, bundleSKUs []string) ([]domainBundle.BundleItem, error) {
	return f.bundleItems, nil
}
func (f *fakeRepo) BundleItemsFor(ctx context.Context, bundleSKU string) ([]domainBundle.BundleItem, error) {
	return f.bundleItemsForSKU, nil
}
func (f *fakeRepo) ComponentAvailability(ctx context.Context, codes []string) ([]domainCatalog.StockCodeRow, error) {
	return f.compAvail, nil
}
func (f *fakeRepo) ComponentAvailabilityFor(ctx context.Context, code string) (int, error) {
	return f.compAvailFor, nil
}
func (f *fakeRepo) AccessoriesForAll(ctx context.Context, skus []string) ([]domainSKU.SKUAccessory, error) {
	return f.accessoriesForAll, nil
}
func (f *fakeRepo) AccessoriesForSKU(ctx context.Context, sku string) ([]domainSKU.SKUAccessory, error) {
	return f.accessoriesForSKU, nil
}
func (f *fakeRepo) AccessoriesByUpperSKU(ctx context.Context, sku string) ([]domainSKU.SKUAccessory, error) {
	return f.accessoriesByUpper, nil
}
func (f *fakeRepo) UsedQtyFor(ctx context.Context, skuID uint, skuUpper string) (int64, error) {
	return f.usedQtyFor, nil
}
func (f *fakeRepo) ResolveUpperTrim(ctx context.Context, codes []string) ([]domainSKU.SKU, error) {
	return f.resolved, nil
}
func (f *fakeRepo) FindActiveFormulas(ctx context.Context, codes []string) ([]domainFormula.InventoryFormula, error) {
	return f.formulas, nil
}
func (f *fakeRepo) AccessoryExists(ctx context.Context, accessorySKU string) (bool, error) {
	return f.accessoryExists, nil
}
func (f *fakeRepo) AccessoryCycleExists(ctx context.Context, accessorySKU, sku string) (bool, error) {
	return f.accessoryCycle, nil
}
func (f *fakeRepo) CreateSKU(ctx context.Context, sku *domainSKU.SKU) error {
	f.createdSKUs = append(f.createdSKUs, *sku)
	return nil
}
func (f *fakeRepo) SaveSKU(ctx context.Context, sku *domainSKU.SKU) error { return nil }
func (f *fakeRepo) DeleteSKU(ctx context.Context, id uint) error {
	f.deletedSKUID = id
	return nil
}
func (f *fakeRepo) LockSKU(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	return f.lockSKU, nil
}
func (f *fakeRepo) CreateStock(ctx context.Context, stock *domainStock.Stock) error {
	f.createdStocks = append(f.createdStocks, *stock)
	return nil
}
func (f *fakeRepo) DeleteStocksBySKUID(ctx context.Context, skuID uint) error { return nil }
func (f *fakeRepo) CreateMovement(ctx context.Context, movement *domainStock.StockMovement) error {
	f.createMovements = append(f.createMovements, *movement)
	return nil
}
func (f *fakeRepo) CreateBundleItem(ctx context.Context, item *domainBundle.BundleItem) error {
	f.createdBundle = append(f.createdBundle, *item)
	return nil
}
func (f *fakeRepo) DeleteBundleItemsByBundleSKU(ctx context.Context, bundleSKU string) error {
	return nil
}
func (f *fakeRepo) CreateAccessory(ctx context.Context, item *domainSKU.SKUAccessory) error {
	f.createdAccs = append(f.createdAccs, *item)
	return nil
}
func (f *fakeRepo) DeleteAccessoriesBySKU(ctx context.Context, sku string) error { return nil }
func (f *fakeRepo) CountReferences(ctx context.Context, sku *domainSKU.SKU) (*domainCatalog.ReferenceCounts, error) {
	return f.refs, nil
}
func (f *fakeRepo) CascadeSKURename(ctx context.Context, oldSKU, newSKU string) error { return nil }

// fakeStats implements ReceiptStatsSource.
type fakeStats struct {
	stats map[uint]domainSKU.SKUBatchReceiptStat
}

func (f *fakeStats) GetReceiptStatsBatch(_ context.Context, skuIDs []uint) (map[uint]domainSKU.SKUBatchReceiptStat, error) {
	return f.stats, nil
}

// fakeTxMgr runs the callback immediately and counts invocations so tests can
// verify per-row transactions.
type fakeTxMgr struct{ calls int }

func (f *fakeTxMgr) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	f.calls++
	return fn(ctx)
}

// ---- List ----

func TestListBuildsRowsWithTikTokReserveAndBundleAvailable(t *testing.T) {
	r := &fakeRepo{
		count: 2,
		search: []domainSKU.SKU{
			{ID: 1, SKU: "PROD-SET", Name: "Set", Category: "Bundle", IsBundle: true},
			{ID: 2, SKU: "ACC-01"},
		},
		sumStocks: []domainCatalog.StockSumRow{
			{SKUID: 1, Quantity: 0, AvailableQty: 0, ReservedQty: 2},
			{SKUID: 2, Quantity: 7, AvailableQty: 7, ReservedQty: 0},
		},
		sumUsed: []domainCatalog.UsedSumRow{
			{SKUID: 2, SKUCode: "ACC-01", Qty: 4},
		},
		mappings: []domainTikTok.SKUMapping{
			{LocalSKU: "ACC-01", TikTokSKU: "TT-ACC01"},
		},
		pending: []domainCatalog.PendingRow{
			{SKU: "ACC-01", Qty: 3},
			{SKU: "TT-ACC01", Qty: 6},
		},
		bundleItems: []domainBundle.BundleItem{
			{BundleSKU: "PROD-SET", ComponentSKU: "ACC-01", Quantity: 3},
		},
		compAvail: []domainCatalog.StockCodeRow{
			{SKUCode: "ACC-01", AvailableQty: 7},
		},
		accessoriesForAll: []domainSKU.SKUAccessory{
			{ID: 9, SKU: "ACC-01", AccessorySKU: "BOX-1", Quantity: 1, Note: "box"},
		},
	}
	r.refs = &domainCatalog.ReferenceCounts{}
	recs, total, err := newTestUsecase(r, &fakeTxMgr{}).List(context.Background(), domainCatalog.ProductQuery{Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Len(t, recs, 2)
	assert.EqualValues(t, 2, total)

	// Bundle row: reserved = 2, available clamped to 0; bundleAvailable = 7/3
	b := recs[0]
	assert.True(t, b.IsBundle)
	assert.Equal(t, "Bundle", b.Type)
	assert.Equal(t, 2, b.ReservedQty)
	assert.Equal(t, 0, b.Available)
	require.NotNil(t, b.BundleAvailable)
	assert.Equal(t, 2, *b.BundleAvailable)

	// Product row: tiktok reserved 3 + mapped TT pending 6, used 4, has accessory
	p := recs[1]
	assert.False(t, p.IsBundle)
	assert.Equal(t, "Finished Product", p.Type)
	assert.Equal(t, 7, p.Stock)
	assert.Equal(t, 9, p.ReservedQty) // 7 - (0 + 9)
	assert.Equal(t, 0, p.Available)
	assert.Equal(t, 4, p.UsedQty)
	assert.Equal(t, "ชิ้น", p.BaseUnit)
	require.Len(t, p.Accessories, 1)
	assert.Equal(t, "BOX-1", p.Accessories[0].AccessorySKU)
}

func TestListCountErrorBecomes500DBError(t *testing.T) {
	r := &fakeRepo{countE: fmt.Errorf("db down")}
	_, _, err := newTestUsecase(r, &fakeTxMgr{}).List(context.Background(), domainCatalog.ProductQuery{})
	require.Error(t, err)
	var dbe *DBError
	require.ErrorAs(t, err, &dbe)
	assert.Equal(t, 500, dbe.Status)
	assert.Equal(t, "Failed to count products: db down", dbe.Message)
}

// ---- Delete ----

func TestDeleteReferencesBlockWithProductInUse(t *testing.T) {
	refs := &domainCatalog.ReferenceCounts{PositiveStock: 3, Movements: 5, OrderItems: 1}
	lock := &domainSKU.SKU{ID: 7, SKU: "USED-1"}
	r := &fakeRepo{findByID: lock, lockSKU: lock, refs: refs}
	err := newTestUsecase(r, &fakeTxMgr{}).DeleteByID(context.Background(), "7")
	require.Error(t, err)
	var appE *appErrors.AppError
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, "PRODUCT_IN_USE", appE.Code)
	assert.Equal(t, 400, appE.StatusCode)
	assert.Contains(t, appE.Message, "ไม่สามารถลบสินค้านี้ได้")
	assert.Contains(t, appE.Message, "มีสต็อกคงเหลือ (3 รายการ)")
	assert.Contains(t, appE.Message, "อยู่ในรายการคำสั่งซื้อ (1 รายการ)")
	// nothing deleted
	assert.Zero(t, r.deletedSKUID)
}

func TestDeleteCleanRowsWhenNoReferences(t *testing.T) {
	lock := &domainSKU.SKU{ID: 7, SKU: "FREE-1"}
	r := &fakeRepo{findByID: lock, lockSKU: lock, refs: &domainCatalog.ReferenceCounts{}}
	tx := &fakeTxMgr{}
	err := newTestUsecase(r, tx).DeleteByID(context.Background(), "7")
	require.NoError(t, err)
	assert.Equal(t, uint(7), r.deletedSKUID)
	assert.Equal(t, 1, tx.calls, "delete should run inside one transaction")
}

func TestDeleteByIDInvalidNumberIsNotFound(t *testing.T) {
	r := &fakeRepo{}
	err := newTestUsecase(r, &fakeTxMgr{}).DeleteByID(context.Background(), "abc")
	var appE *appErrors.AppError
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, "NOT_FOUND", appE.Code)
}

// ---- Create ----

func TestCreateValidationMessages(t *testing.T) {
	tx := &fakeTxMgr{}
	u := newTestUsecase(&fakeRepo{}, tx)

	_, err := u.Create(context.Background(), CreateInput{SKU: "  ", Name: "X"})
	require.EqualError(t, err, "SKU is required")

	_, err = u.Create(context.Background(), CreateInput{SKU: "A-1"})
	// (behavior parity with the original handler: name is checked after trim-free
	// raw value, i.e. only an empty string triggers this branch)
	require.EqualError(t, err, "Product name is required")
	assert.Zero(t, tx.calls, "validation failure must not open transactions")
}

func TestCreateForcesNonBundleAndCreatesOpeningStock(t *testing.T) {
	r := &fakeRepo{}
	tx := &fakeTxMgr{}
	rec, err := newTestUsecase(r, tx).Create(context.Background(), CreateInput{
		SKU:         " a-1 ",
		Name:        "Accessory A",
		IsBundle:    true, // spec: forced false
		RetailPrice: 99,
		Components:  []ComponentLine{{ComponentSKU: "X-1", Quantity: 2}},
	})
	require.NoError(t, err)
	require.Len(t, r.createdSKUs, 1)
	created := r.createdSKUs[0]
	assert.False(t, created.IsBundle, "isBundle is forced to false")
	assert.Equal(t, "A-1", created.SKU)
	assert.Equal(t, "active", created.Status)
	// bundle components must NOT be created for a forced non-bundle SKU
	assert.Empty(t, r.createdBundle)
	// opening stock is created with 0 qty (no initial quantity)
	require.Len(t, r.createdStocks, 1)
	assert.Equal(t, 0, r.createdStocks[0].Quantity)
	assert.Empty(t, r.createMovements, "no movement when initial stock is 0")
	assert.True(t, rec.IsActive)
}

func TestCreateNegativeInitialStockClampedToZero(t *testing.T) {
	r := &fakeRepo{}
	_, err := newTestUsecase(r, &fakeTxMgr{}).Create(context.Background(), CreateInput{SKU: "A-1", Name: "A", InitialQuantity: -5})
	require.NoError(t, err)
	require.Len(t, r.createdStocks, 1)
	assert.Equal(t, 0, r.createdStocks[0].Quantity)
	assert.Empty(t, r.createMovements)
}

// ---- Import ----

func TestImportDuplicateInsideFileReportsFirstRow(t *testing.T) {
	r := &fakeRepo{} // ExistsUpper=false (db clean)
	u := newTestUsecase(r, &fakeTxMgr{})
	lines := []ImportLine{
		{Row: 2, SKU: "A-1", Name: "First"},
		{Row: 3, SKU: "A-1", Name: "Dup"},
	}
	summary := u.Import(context.Background(), 2, lines)
	require.NotNil(t, summary)
	assert.Equal(t, 1, summary.Created)
	assert.Equal(t, 1, summary.Failed)
	require.Len(t, summary.Errors, 1)
	assert.Equal(t, 3, summary.Errors[0].Row)
	assert.Equal(t, "A-1", summary.Errors[0].SKU)
	assert.True(t, strings.Contains(summary.Errors[0].Error, "ซ้ำกับแถวที่ 2"), "dup error names the first occurrence row")
}

func TestImportDuplicateInDatabaseRejected(t *testing.T) {
	r := &fakeRepo{existsUpper: true}
	u := newTestUsecase(r, &fakeTxMgr{})
	summary := u.Import(context.Background(), 1, []ImportLine{{Row: 2, SKU: "A-1", Name: "Existing"}})
	assert.Equal(t, 1, summary.Failed)
	assert.Zero(t, summary.Created)
	require.Len(t, summary.Errors, 1)
	assert.True(t, strings.Contains(summary.Errors[0].Error, "มีอยู่ในระบบแล้ว (ห้ามซ้ำ)"))
	assert.Zero(t, r.createdSKUs, "rejected rows must not reach CreateSKU")
}

func TestImportPerRowTransactionAndValueNormalization(t *testing.T) {
	r := &fakeRepo{}
	tx := &fakeTxMgr{}
	u := newTestUsecase(r, tx)
	lines := []ImportLine{
		{Row: 2, SKU: "A-1", Name: "One", Price: "9.5", Cost: "4", Stock: "-3", Reorder: "-1"},
		{Row: 3, SKU: "a-2", Name: "Two", Stock: "5", Reorder: "3"},
		{Row: 4, SKU: "B-1", Name: "Three"},
	}
	summary := u.Import(context.Background(), 3, lines)
	assert.Equal(t, 3, summary.Created)
	assert.Zero(t, summary.Failed)
	assert.Equal(t, 3, tx.calls, "one transaction per row, not per file")
	assert.Len(t, r.createdStocks, 3)

	bySku := map[string]domainSKU.SKU{}
	for _, s := range r.createdSKUs {
		bySku[s.SKU] = s
	}
	assert.Equal(t, 10, bySku["A-1"].ReorderPoint, "negative reorder is floored to 10")
	assert.Equal(t, 3, bySku["A-2"].ReorderPoint)
	assert.Equal(t, 0, bySku["B-1"].ReorderPoint, "unparsable/missing reorder stays 0 (no default applied)")
	assert.False(t, bySku["A-2"].IsBundle)

	stockByCode := map[string]int{}
	for _, st := range r.createdStocks {
		stockByCode[st.SKUCode] = st.Quantity
	}
	assert.Equal(t, 0, stockByCode["A-1"], "negative stock clamped to 0")
	assert.Equal(t, 5, stockByCode["A-2"], "stock parsed from the sheet reaches the created stock row")
	assert.Equal(t, 0, stockByCode["B-1"], "missing stock stays 0")
}

// ---- Resolve ----

func TestResolveFallsBackToActiveFormulas(t *testing.T) {
	r := &fakeRepo{
		resolved: []domainSKU.SKU{{ID: 5, SKU: "SKU-1", Name: "Master SKU"}},
		formulas: []domainFormula.InventoryFormula{{ID: 11, Code: "FI-01", Name: "Formula One"}},
	}
	recs, err := newTestUsecase(r, &fakeTxMgr{}).Resolve(context.Background(), []string{"SKU-1", "FI-01", "MISSING-X"})
	require.NoError(t, err)
	require.Len(t, recs, 3)
	assert.True(t, recs[0].Found)
	assert.Equal(t, uint(5), recs[0].Id)
	assert.Equal(t, "SKU-1", recs[0].Sku)
	assert.True(t, recs[1].Found)
	assert.Equal(t, "FI-01", recs[1].Sku)
	assert.Equal(t, uint(11), recs[1].Id)
	assert.False(t, recs[2].Found)
	assert.Equal(t, "MISSING-X", recs[2].Sku)
}

// ---- Update/accessory payload validation ----

func TestUpdateByIDSkuRenameConflictKeepsThaiMessage(t *testing.T) {
	r := &fakeRepo{
		findByID:        &domainSKU.SKU{ID: 3, SKU: "OLD-1"},
		existsUpperExcl: true,
	}
	_, err := newTestUsecase(r, &fakeTxMgr{}).UpdateByID(context.Background(), "3", UpdateInput{SKU: strPtr("NEW-1")})
	var appE *appErrors.AppError
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, "SKU_ALREADY_EXISTS", appE.Code)
	assert.Equal(t, 400, appE.StatusCode)
	assert.Contains(t, appE.Message, "รหัส SKU 'NEW-1' ถูกใช้งานแล้ว")
}

func TestUpdateByIDNotFoundForBadNumber(t *testing.T) {
	r := &fakeRepo{}
	_, err := newTestUsecase(r, &fakeTxMgr{}).UpdateByID(context.Background(), "zz", UpdateInput{})
	var appE *appErrors.AppError
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, "NOT_FOUND", appE.Code)
}

func TestAccessoryValidationCodes(t *testing.T) {
	r := &fakeRepo{}
	u := newTestUsecase(r, &fakeTxMgr{})
	ctx := context.Background()

	// self binding
	err := u.validateAccessories(ctx, "SELF-1", []AccessoryLine{{AccessorySKU: "SELF-1", Quantity: 1}})
	require.EqualError(t, err, "สินค้าไม่สามารถผูกตนเองเป็น Accessory ได้")

	// quantity must be positive (Thai message, code BAD_REQUEST)
	r2 := &fakeRepo{accessoryExists: true}
	u2 := newTestUsecase(r2, &fakeTxMgr{})
	err = u2.validateAccessories(ctx, "P-1", []AccessoryLine{{AccessorySKU: "A-1", Quantity: 0}})
	var appE *appErrors.AppError
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, "BAD_REQUEST", appE.Code)
	assert.True(t, strings.Contains(appE.Message, "ต้องมากกว่า 0"))

	// not-found accessory keeps the ACCESSORY_NOT_FOUND code
	r3 := &fakeRepo{}
	u3 := newTestUsecase(r3, &fakeTxMgr{})
	err = u3.validateAccessories(ctx, "P-1", []AccessoryLine{{AccessorySKU: "NOPE", Quantity: 1}})
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, "ACCESSORY_NOT_FOUND", appE.Code)
	assert.True(t, strings.Contains(appE.Message, "ไม่พบ SKU Accessory 'NOPE' ในระบบ"))

	// cycle keeps ACCESSORY_CYCLE
	r4 := &fakeRepo{accessoryExists: true, accessoryCycle: true}
	u4 := newTestUsecase(r4, &fakeTxMgr{})
	err = u4.validateAccessories(ctx, "P-1", []AccessoryLine{{AccessorySKU: "A-1", Quantity: 1}})
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, "ACCESSORY_CYCLE", appE.Code)
}

func TestAccessoryQtyFromQtyFieldAccepted(t *testing.T) {
	r := &fakeRepo{accessoryExists: true}
	u := newTestUsecase(r, &fakeTxMgr{})
	assert.Nil(t, u.validateAccessories(context.Background(), "P-1", []AccessoryLine{{AccessorySKU: "A-1", Qty: 2}}))
}

// strPtr is a small helper for pointer fields in UpdateInput.
func strPtr(s string) *string { return &s }

// compile-time guards so test helpers stay in sync with production contracts.
var (
	_ domainCatalog.Repository = (*fakeRepo)(nil)
	_ pkgDatabase.TxManager    = (*fakeTxMgr)(nil)
	_ ReceiptStatsSource       = (*fakeStats)(nil)
)
