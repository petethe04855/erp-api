package order_test

import (
	"context"
	"fmt"
	"testing"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecaseOrder "chawy-erp-api/internal/usecase/order"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTxManager struct{}

func (f *fakeTxManager) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type mockOrderRepo struct {
	orders map[uint]*domainOrder.Order
	nextID uint
}

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{orders: make(map[uint]*domainOrder.Order), nextID: 1}
}

func (m *mockOrderRepo) Create(ctx context.Context, o *domainOrder.Order) error {
	o.ID = m.nextID
	m.nextID++
	cp := *o
	m.orders[o.ID] = &cp
	return nil
}

func (m *mockOrderRepo) FindByID(ctx context.Context, id uint) (*domainOrder.Order, error) {
	o, ok := m.orders[id]
	if !ok {
		return nil, nil
	}
	cp := *o
	return &cp, nil
}

func (m *mockOrderRepo) FindByIDForUpdate(ctx context.Context, id uint) (*domainOrder.Order, error) {
	return m.FindByID(ctx, id)
}

func (m *mockOrderRepo) FindByOrderNo(ctx context.Context, orderNo string) (*domainOrder.Order, error) {
	for _, o := range m.orders {
		if o.OrderNo == orderNo {
			cp := *o
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockOrderRepo) FindAll(ctx context.Context, q domainOrder.Query) ([]domainOrder.Order, int64, error) {
	var res []domainOrder.Order
	for _, o := range m.orders {
		res = append(res, *o)
	}
	return res, int64(len(res)), nil
}

func (m *mockOrderRepo) UpdateStatus(ctx context.Context, id uint, status domainOrder.Status) error {
	if o, ok := m.orders[id]; ok {
		o.Status = status
		return nil
	}
	return appErrors.ErrNotFound
}

func (m *mockOrderRepo) Update(ctx context.Context, o *domainOrder.Order) error {
	m.orders[o.ID] = o
	return nil
}

type mockSKURepo struct {
	skus map[string]*domainSKU.SKU
}

func newMockSKURepo() *mockSKURepo {
	return &mockSKURepo{skus: make(map[string]*domainSKU.SKU)}
}

func (m *mockSKURepo) Create(ctx context.Context, sku *domainSKU.SKU) error {
	m.skus[sku.SKU] = sku
	return nil
}

func (m *mockSKURepo) FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	for _, s := range m.skus {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSKURepo) FindBySKU(ctx context.Context, code string) (*domainSKU.SKU, error) {
	s, ok := m.skus[code]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockSKURepo) FindAll(ctx context.Context, query domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	return nil, 0, nil
}

func (m *mockSKURepo) Update(ctx context.Context, sku *domainSKU.SKU) error {
	m.skus[sku.SKU] = sku
	return nil
}

func (m *mockSKURepo) Delete(ctx context.Context, id uint) error {
	return nil
}

func (m *mockSKURepo) ExistsBySKU(ctx context.Context, code string) (bool, error) {
	_, ok := m.skus[code]
	return ok, nil
}

type mockBundleRepo struct {
	bundles map[string][]domainBundle.BundleItem
}

func newMockBundleRepo() *mockBundleRepo {
	return &mockBundleRepo{bundles: make(map[string][]domainBundle.BundleItem)}
}

func (m *mockBundleRepo) GetItemsByBundleSKU(ctx context.Context, bundleSKU string) ([]domainBundle.BundleItem, error) {
	return m.bundles[bundleSKU], nil
}

func (m *mockBundleRepo) SaveItems(ctx context.Context, bundleSKU string, items []domainBundle.BundleItem) error {
	m.bundles[bundleSKU] = items
	return nil
}

func (m *mockBundleRepo) DeleteItem(ctx context.Context, id uint) error {
	return nil
}

type mockStockRepo struct {
	stocks    map[string]*domainStock.Stock
	movements []*domainStock.StockMovement
}

func newMockStockRepo() *mockStockRepo {
	return &mockStockRepo{stocks: make(map[string]*domainStock.Stock)}
}

func (m *mockStockRepo) key(skuID, whID uint) string {
	return fmt.Sprintf("%d-%d", skuID, whID)
}

func (m *mockStockRepo) GetBySKUID(ctx context.Context, skuID, whID uint) (*domainStock.Stock, error) {
	s, ok := m.stocks[m.key(skuID, whID)]
	if !ok {
		return nil, nil
	}
	cp := *s
	return &cp, nil
}

func (m *mockStockRepo) GetBySKUIDForUpdate(ctx context.Context, skuID, whID uint) (*domainStock.Stock, error) {
	return m.GetBySKUID(ctx, skuID, whID)
}

func (m *mockStockRepo) FindAll(ctx context.Context, query domainStock.Query) ([]domainStock.Stock, int64, error) {
	return nil, 0, nil
}

func (m *mockStockRepo) UpdateQuantity(ctx context.Context, skuID, whID uint, delta int) (*domainStock.Stock, error) {
	k := m.key(skuID, whID)
	stk, ok := m.stocks[k]
	if !ok {
		stk = &domainStock.Stock{SKUID: skuID, WarehouseID: whID}
		m.stocks[k] = stk
	}
	stk.Quantity += delta
	stk.AvailableQty = stk.Quantity - stk.ReservedQty
	cp := *stk
	return &cp, nil
}

func (m *mockStockRepo) ReserveStock(ctx context.Context, skuID, whID uint, qty int) (*domainStock.Stock, error) {
	k := m.key(skuID, whID)
	stk, ok := m.stocks[k]
	if !ok {
		stk = &domainStock.Stock{SKUID: skuID, WarehouseID: whID}
		m.stocks[k] = stk
	}
	stk.ReservedQty += qty
	stk.AvailableQty = stk.Quantity - stk.ReservedQty
	cp := *stk
	return &cp, nil
}

func (m *mockStockRepo) ReleaseStock(ctx context.Context, skuID, whID uint, qty int) (*domainStock.Stock, error) {
	k := m.key(skuID, whID)
	stk, ok := m.stocks[k]
	if !ok {
		stk = &domainStock.Stock{SKUID: skuID, WarehouseID: whID}
		m.stocks[k] = stk
	}
	stk.ReservedQty -= qty
	if stk.ReservedQty < 0 {
		stk.ReservedQty = 0
	}
	stk.AvailableQty = stk.Quantity - stk.ReservedQty
	cp := *stk
	return &cp, nil
}

func (m *mockStockRepo) CreateMovement(ctx context.Context, movement *domainStock.StockMovement) error {
	m.movements = append(m.movements, movement)
	return nil
}

func (m *mockStockRepo) GetMovements(ctx context.Context, skuID uint, page, limit int) ([]domainStock.StockMovement, int64, error) {
	return nil, 0, nil
}

func TestOrderCreate_SucceedsAndReservesStock(t *testing.T) {
	orderRepo := newMockOrderRepo()
	skuRepo := newMockSKURepo()
	bundleRepo := newMockBundleRepo()
	stockRepo := newMockStockRepo()
	txMgr := &fakeTxManager{}

	skuRepo.skus["SKU-01"] = &domainSKU.SKU{ID: 10, SKU: "SKU-01", Name: "Widget A", Price: 100}
	stockRepo.stocks["10-1"] = &domainStock.Stock{ID: 1, SKUID: 10, WarehouseID: 1, Quantity: 20, ReservedQty: 0, AvailableQty: 20}

	uc := usecaseOrder.NewOrderUsecaseWithTx(nil, orderRepo, skuRepo, bundleRepo, stockRepo, txMgr)

	order, err := uc.Create(context.Background(), usecaseOrder.CreateOrderInput{
		CustomerName: "Customer A",
		Items: []usecaseOrder.CreateItemInput{
			{SKU: "SKU-01", Quantity: 5, Price: 100},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	assert.Equal(t, domainOrder.StatusPending, order.Status)

	// Check reserved stock
	stk := stockRepo.stocks["10-1"]
	assert.Equal(t, 5, stk.ReservedQty)
	assert.Equal(t, 15, stk.AvailableQty)
	assert.Len(t, stockRepo.movements, 1)
	assert.Equal(t, domainStock.MovementReserve, stockRepo.movements[0].Type)
}

func TestOrderCreate_FailsWhenInsufficientStock(t *testing.T) {
	orderRepo := newMockOrderRepo()
	skuRepo := newMockSKURepo()
	bundleRepo := newMockBundleRepo()
	stockRepo := newMockStockRepo()
	txMgr := &fakeTxManager{}

	skuRepo.skus["SKU-01"] = &domainSKU.SKU{ID: 10, SKU: "SKU-01", Name: "Widget A", Price: 100}
	stockRepo.stocks["10-1"] = &domainStock.Stock{ID: 1, SKUID: 10, WarehouseID: 1, Quantity: 5, ReservedQty: 2, AvailableQty: 3}

	uc := usecaseOrder.NewOrderUsecaseWithTx(nil, orderRepo, skuRepo, bundleRepo, stockRepo, txMgr)

	_, err := uc.Create(context.Background(), usecaseOrder.CreateOrderInput{
		CustomerName: "Customer A",
		Items: []usecaseOrder.CreateItemInput{
			{SKU: "SKU-01", Quantity: 5, Price: 100},
		},
	})

	require.Error(t, err)
	var appErr *appErrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, "INSUFFICIENT_STOCK", appErr.Code)
	assert.Equal(t, 409, appErr.StatusCode)
	assert.Contains(t, err.Error(), "พร้อมขาย 3")

	// Stock should not change
	stk := stockRepo.stocks["10-1"]
	assert.Equal(t, 2, stk.ReservedQty)
}

func TestOrderCreate_BundleExplodesAndReservesComponents(t *testing.T) {
	orderRepo := newMockOrderRepo()
	skuRepo := newMockSKURepo()
	bundleRepo := newMockBundleRepo()
	stockRepo := newMockStockRepo()
	txMgr := &fakeTxManager{}

	skuRepo.skus["BUNDLE-01"] = &domainSKU.SKU{ID: 100, SKU: "BUNDLE-01", Name: "Bundle Kit", Price: 500, IsBundle: true}
	skuRepo.skus["COMP-A"] = &domainSKU.SKU{ID: 101, SKU: "COMP-A", Name: "Component A"}
	skuRepo.skus["COMP-B"] = &domainSKU.SKU{ID: 102, SKU: "COMP-B", Name: "Component B"}

	bundleRepo.bundles["BUNDLE-01"] = []domainBundle.BundleItem{
		{BundleSKU: "BUNDLE-01", ComponentSKU: "COMP-A", Quantity: 2},
		{BundleSKU: "BUNDLE-01", ComponentSKU: "COMP-B", Quantity: 3},
	}

	stockRepo.stocks["101-1"] = &domainStock.Stock{ID: 1, SKUID: 101, WarehouseID: 1, Quantity: 20, ReservedQty: 0, AvailableQty: 20}
	stockRepo.stocks["102-1"] = &domainStock.Stock{ID: 2, SKUID: 102, WarehouseID: 1, Quantity: 30, ReservedQty: 0, AvailableQty: 30}

	uc := usecaseOrder.NewOrderUsecaseWithTx(nil, orderRepo, skuRepo, bundleRepo, stockRepo, txMgr)

	// Order 2 bundles: requires 2*2 = 4 COMP-A and 2*3 = 6 COMP-B
	order, err := uc.Create(context.Background(), usecaseOrder.CreateOrderInput{
		CustomerName: "Customer Bundle",
		Items: []usecaseOrder.CreateItemInput{
			{SKU: "BUNDLE-01", Quantity: 2, Price: 500},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, order)

	stkA := stockRepo.stocks["101-1"]
	assert.Equal(t, 4, stkA.ReservedQty)
	assert.Equal(t, 16, stkA.AvailableQty)

	stkB := stockRepo.stocks["102-1"]
	assert.Equal(t, 6, stkB.ReservedQty)
	assert.Equal(t, 24, stkB.AvailableQty)
	assert.Len(t, stockRepo.movements, 2)
}

func TestOrderCancel_ReleasesReservedStock(t *testing.T) {
	orderRepo := newMockOrderRepo()
	skuRepo := newMockSKURepo()
	bundleRepo := newMockBundleRepo()
	stockRepo := newMockStockRepo()
	txMgr := &fakeTxManager{}

	skuRepo.skus["SKU-01"] = &domainSKU.SKU{ID: 10, SKU: "SKU-01", Name: "Widget A", Price: 100}
	stockRepo.stocks["10-1"] = &domainStock.Stock{ID: 1, SKUID: 10, WarehouseID: 1, Quantity: 20, ReservedQty: 5, AvailableQty: 15}

	orderRepo.orders[1] = &domainOrder.Order{
		ID:      1,
		OrderNo: "SO-20260101-0001",
		Status:  domainOrder.StatusPending,
		Items: []domainOrder.OrderItem{
			{SKU: "SKU-01", Quantity: 5, Price: 100},
		},
	}

	uc := usecaseOrder.NewOrderUsecaseWithTx(nil, orderRepo, skuRepo, bundleRepo, stockRepo, txMgr)

	cancelled, err := uc.CancelOrder(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, domainOrder.StatusCancelled, cancelled.Status)

	// Reserved stock must be released
	stk := stockRepo.stocks["10-1"]
	assert.Equal(t, 0, stk.ReservedQty)
	assert.Equal(t, 20, stk.AvailableQty)
	require.Len(t, stockRepo.movements, 1)
	assert.Equal(t, domainStock.MovementRelease, stockRepo.movements[0].Type)
}
