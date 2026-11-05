package purchasing_test

import (
	"context"
	"errors"
	"testing"

	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecasePurchasing "chawy-erp-api/internal/usecase/purchasing"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wsPurchRepo wraps the shared fake with supplier bookkeeping so the workspace
// purchase-order flow (resolve / create supplier) can be exercised.
type wsPurchRepo struct {
	*fakePurchRepo
	suppliers      map[string]*domainPurchasing.Supplier
	nextSupID      uint
	pos            map[uint]*domainPurchasing.PurchaseOrder
	nextPOID       uint
	movements      []domainStock.StockMovement
	lastIssueQuery domainPurchasing.GoodsIssueQuery
}

func newWSRepo() *wsPurchRepo {
	return &wsPurchRepo{
		fakePurchRepo: &fakePurchRepo{pos: map[uint]*domainPurchasing.PurchaseOrder{}},
		suppliers:     map[string]*domainPurchasing.Supplier{},
		pos:           map[uint]*domainPurchasing.PurchaseOrder{},
		nextSupID:     1,
		nextPOID:      1,
	}
}

func (f *wsPurchRepo) FindSupplierByID(ctx context.Context, id uint) (*domainPurchasing.Supplier, error) {
	for _, s := range f.suppliers {
		if s.ID == id {
			cp := *s
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *wsPurchRepo) FindAllSuppliers(ctx context.Context, q domainPurchasing.SupplierQuery) ([]domainPurchasing.Supplier, int64, error) {
	var out []domainPurchasing.Supplier
	for _, s := range f.suppliers {
		out = append(out, *s)
	}
	return out, int64(len(out)), nil
}

func (f *wsPurchRepo) CreateSupplier(ctx context.Context, s *domainPurchasing.Supplier) error {
	s.ID = f.nextSupID
	f.nextSupID++
	cp := *s
	f.suppliers[s.Name] = &cp
	return nil
}

func (f *wsPurchRepo) CreatePO(ctx context.Context, po *domainPurchasing.PurchaseOrder) error {
	po.ID = f.nextPOID
	f.nextPOID++
	cp := *po
	f.pos[po.ID] = &cp
	return nil
}

func (f *wsPurchRepo) UpdatePO(ctx context.Context, po *domainPurchasing.PurchaseOrder) error {
	if stored, ok := f.pos[po.ID]; ok {
		stored.Status = po.Status
		stored.UpdatedAt = po.UpdatedAt
	}
	return nil
}

func (f *wsPurchRepo) FindPOByID(ctx context.Context, id uint) (*domainPurchasing.PurchaseOrder, error) {
	if po, ok := f.pos[id]; ok {
		cp := *po
		return &cp, nil
	}
	return nil, nil
}

func (f *wsPurchRepo) FindAllGoodsIssues(ctx context.Context, q domainPurchasing.GoodsIssueQuery) ([]domainStock.StockMovement, int64, error) {
	f.lastIssueQuery = q
	out := make([]domainStock.StockMovement, 0, len(f.movements))
	out = append(out, f.movements...)
	return out, int64(len(out)), nil
}

func (f *wsPurchRepo) FindGoodsIssueByID(ctx context.Context, id uint) (*domainStock.StockMovement, error) {
	for _, m := range f.movements {
		if m.ID == id {
			cp := m
			return &cp, nil
		}
	}
	return nil, nil
}

// Creating a purchase order from the workspace screen resolves (or registers)
// the supplier by name and computes totals / ETA on the backend.
func TestCreatePurchaseOrder_ResolvesSupplierAndComputesTotals(t *testing.T) {
	repo := newWSRepo()
	existing := &domainPurchasing.Supplier{ID: 7, Name: "ACME", Status: "active"}
	repo.suppliers["acme"] = existing
	repo.suppliers["ACME"] = existing

	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, repo,
		&fakeSKURepo{items: map[string]*domainSKU.SKU{}},
		&fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}, inlineTxMgrP{})

	po, err := uc.CreatePurchaseOrder(context.Background(), usecasePurchasing.CreatePurchaseOrderInput{
		Supplier: "ACME",
		Note:     "restock",
		Items: []usecasePurchasing.CreatePurchaseOrderItem{
			{SKU: "SKU-A", Name: "A", Quantity: 4, UnitCost: 25},
			{SKU: "SKU-B", Name: "B", Qty: 2, UnitCost: 10},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, uint(7), po.SupplierID)
	assert.Equal(t, "ACME", po.SupplierName)
	assert.Equal(t, domainPurchasing.StatusPending, po.Status)
	assert.Equal(t, float64(25*4+10*2), po.TotalAmount)
	require.Len(t, po.Items, 2)
	assert.Equal(t, 4, po.Items[0].Quantity)
	assert.Equal(t, 2, po.Items[1].Quantity) // falls back to qty when quantity missing
	// Default ETA is computed on the backend (7 days); with no explicit
	// etaDate nothing is appended to the note (original behaviour).
	assert.Equal(t, "restock", po.Note)
	assert.NotEmpty(t, po.ETADate)
	assert.NotEmpty(t, po.ETADate)
	assert.NotEmpty(t, po.PONo)
	require.Len(t, repo.pos, 1)
}

// An explicit etaDate is mirrored into the note with the ETA: prefix.
func TestCreatePurchaseOrder_ExplicitETAGoesIntoNote(t *testing.T) {
	repo := newWSRepo()
	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, repo,
		&fakeSKURepo{items: map[string]*domainSKU.SKU{}},
		&fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}, inlineTxMgrP{})

	po, err := uc.CreatePurchaseOrder(context.Background(), usecasePurchasing.CreatePurchaseOrderInput{
		Supplier: "ACME",
		Note:     "restock",
		ETADate:  "2026-12-01",
		Items:    []usecasePurchasing.CreatePurchaseOrderItem{{SKU: "SKU-A", Quantity: 1, UnitCost: 5}},
	})
	require.NoError(t, err)
	assert.Equal(t, "2026-12-01", po.ETADate)
	assert.Equal(t, "restock ETA:2026-12-01", po.Note)
}

// A supplier given only by name is created with an auto code.
func TestCreatePurchaseOrder_CreatesNewSupplier(t *testing.T) {
	repo := newWSRepo()
	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, repo,
		&fakeSKURepo{items: map[string]*domainSKU.SKU{}},
		&fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}, inlineTxMgrP{})

	po, err := uc.CreatePurchaseOrder(context.Background(), usecasePurchasing.CreatePurchaseOrderInput{
		Supplier: "NEW SUP",
		Items:    []usecasePurchasing.CreatePurchaseOrderItem{{SKU: "SKU-A", Quantity: 1, UnitCost: 5}},
	})
	require.NoError(t, err)
	assert.Equal(t, uint(1), po.SupplierID)
	assert.Equal(t, "NEW SUP", po.SupplierName)
	assert.Contains(t, repo.suppliers, "NEW SUP")
}

// Validation failures keep the historical messages.
func TestCreatePurchaseOrder_Validation(t *testing.T) {
	repo := newWSRepo()
	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, repo,
		&fakeSKURepo{items: map[string]*domainSKU.SKU{}},
		&fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}, inlineTxMgrP{})

	// No items
	_, err := uc.CreatePurchaseOrder(context.Background(), usecasePurchasing.CreatePurchaseOrderInput{})
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 400, appErr.StatusCode)
	assert.Equal(t, "Purchase order must contain at least one item", appErr.Message)

	// Non-positive quantity
	_, err = uc.CreatePurchaseOrder(context.Background(), usecasePurchasing.CreatePurchaseOrderInput{
		Items: []usecasePurchasing.CreatePurchaseOrderItem{{SKU: "SKU-A", Quantity: 0, UnitCost: 5}},
	})
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "Item quantity for SKU SKU-A must be greater than 0", appErr.Message)

	// Negative unit cost
	_, err = uc.CreatePurchaseOrder(context.Background(), usecasePurchasing.CreatePurchaseOrderInput{
		Items: []usecasePurchasing.CreatePurchaseOrderItem{{SKU: "SKU-A", Quantity: 1, UnitCost: -1}},
	})
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "Item unit cost for SKU SKU-A cannot be negative", appErr.Message)

	// Nothing was persisted
	assert.Len(t, repo.pos, 0)
}

// Updating a missing PO keeps the historical 404 message.
func TestUpdatePOStatus_NotFound(t *testing.T) {
	repo := newWSRepo()
	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, repo,
		&fakeSKURepo{items: map[string]*domainSKU.SKU{}},
		&fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}, inlineTxMgrP{})

	_, err := uc.UpdatePOStatus(context.Background(), 99, "APPROVED")
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 404, appErr.StatusCode)
	assert.Equal(t, "Purchase order not found", appErr.Message)
}

// A real status change upper-cases the value and persists it.
func TestUpdatePOStatus_AppliesUppercaseStatus(t *testing.T) {
	repo := newWSRepo()
	repo.pos[1] = &domainPurchasing.PurchaseOrder{ID: 1, PONo: "PO-TEST-10", Status: domainPurchasing.StatusPending}
	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, repo,
		&fakeSKURepo{items: map[string]*domainSKU.SKU{}},
		&fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}, inlineTxMgrP{})

	po, err := uc.UpdatePOStatus(context.Background(), 1, "approved")
	require.NoError(t, err)
	assert.Equal(t, domainPurchasing.StatusApproved, po.Status)
	assert.Equal(t, domainPurchasing.StatusApproved, repo.pos[1].Status)
}

// List/Get passthroughs keep the workspace list clamping and messages.
func TestListGoodsIssues_ClampsAndReturns(t *testing.T) {
	repo := newWSRepo()
	repo.movements = []domainStock.StockMovement{
		{ID: 1, SKUCode: "SKU-A", Quantity: 2, Type: domainStock.MovementOut},
	}
	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, repo,
		&fakeSKURepo{items: map[string]*domainSKU.SKU{}},
		&fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}, inlineTxMgrP{})

	items, total, err := uc.ListGoodsIssues(context.Background(), domainPurchasing.GoodsIssueQuery{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, items, 1)
	assert.Equal(t, 1, repo.lastIssueQuery.Page)
	assert.Equal(t, 50, repo.lastIssueQuery.Limit)

	m, err := uc.GetGoodsIssue(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, "SKU-A", m.SKUCode)

	_, err = uc.GetGoodsIssue(context.Background(), 404)
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 404, appErr.StatusCode)
	assert.Equal(t, "Goods issue not found", appErr.Message)
}
