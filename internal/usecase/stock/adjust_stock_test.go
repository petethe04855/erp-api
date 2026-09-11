package stock_test

import (
	"context"
	"errors"
	"testing"

	domainStock "chawy-erp-api/internal/domain/stock"
	usecaseStock "chawy-erp-api/internal/usecase/stock"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
)

// ---- fakes ----

type fakeStockRepo struct {
	stocks       map[uint]*domainStock.Stock // key: warehouse
	movements    []domainStock.StockMovement
	failMovement bool
}

func newFakeStockRepo(s *domainStock.Stock) *fakeStockRepo {
	f := &fakeStockRepo{stocks: map[uint]*domainStock.Stock{}}
	if s != nil {
		f.stocks[s.WarehouseID] = s
	}
	return f
}

func (f *fakeStockRepo) GetBySKUID(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error) {
	if s, ok := f.stocks[warehouseID]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeStockRepo) GetBySKUIDForUpdate(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error) {
	return f.GetBySKUID(ctx, skuID, warehouseID)
}

func (f *fakeStockRepo) FindAll(ctx context.Context, q domainStock.Query) ([]domainStock.Stock, int64, error) {
	return nil, 0, nil
}

func (f *fakeStockRepo) UpdateQuantity(ctx context.Context, skuID, warehouseID uint, delta int) (*domainStock.Stock, error) {
	s, ok := f.stocks[warehouseID]
	if !ok {
		s = &domainStock.Stock{SKUID: skuID, WarehouseID: warehouseID}
		f.stocks[warehouseID] = s
	}
	s.Quantity += delta
	s.AvailableQty = s.Quantity - s.ReservedQty
	return s, nil
}

func (f *fakeStockRepo) ReserveStock(ctx context.Context, skuID, warehouseID uint, qty int) (*domainStock.Stock, error) {
	s, ok := f.stocks[warehouseID]
	if !ok {
		s = &domainStock.Stock{SKUID: skuID, WarehouseID: warehouseID}
		f.stocks[warehouseID] = s
	}
	s.ReservedQty += qty
	s.AvailableQty = s.Quantity - s.ReservedQty
	return s, nil
}

func (f *fakeStockRepo) ReleaseStock(ctx context.Context, skuID, warehouseID uint, qty int) (*domainStock.Stock, error) {
	s, ok := f.stocks[warehouseID]
	if !ok {
		s = &domainStock.Stock{SKUID: skuID, WarehouseID: warehouseID}
		f.stocks[warehouseID] = s
	}
	s.ReservedQty -= qty
	if s.ReservedQty < 0 {
		s.ReservedQty = 0
	}
	s.AvailableQty = s.Quantity - s.ReservedQty
	return s, nil
}

func (f *fakeStockRepo) CreateMovement(ctx context.Context, m *domainStock.StockMovement) error {
	if f.failMovement {
		return errors.New("movement write failed")
	}
	f.movements = append(f.movements, *m)
	return nil
}

func (f *fakeStockRepo) GetMovements(ctx context.Context, skuID uint, page, limit int) ([]domainStock.StockMovement, int64, error) {
	return f.movements, int64(len(f.movements)), nil
}

// Snapshot/Restore emulate DB rollback for tests.
func (f *fakeStockRepo) Snapshot() (map[uint]domainStock.Stock, int) {
	snap := make(map[uint]domainStock.Stock, len(f.stocks))
	for k, v := range f.stocks {
		snap[k] = *v
	}
	return snap, len(f.movements)
}

func (f *fakeStockRepo) Restore(snap map[uint]domainStock.Stock, movCount int) {
	for k, v := range snap {
		cp := v
		f.stocks[k] = &cp
	}
	f.movements = f.movements[:movCount]
}

type inlineTxMgr struct{}

func (inlineTxMgr) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

// rollbackTxMgr restores repo state when fn fails, emulating a DB rollback.
type rollbackTxMgr struct{ repo *fakeStockRepo }

func (m rollbackTxMgr) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	snap, movCount := m.repo.Snapshot()
	if err := fn(ctx); err != nil {
		m.repo.Restore(snap, movCount)
		return err
	}
	return nil
}

// ---- tests ----

// FULL-08: adjusting stock to a value below the reserved quantity must be
// rejected instead of producing a negative available quantity.
func TestAdjustStock_RejectsBelowReserved(t *testing.T) {
	repo := newFakeStockRepo(&domainStock.Stock{
		SKUID:        1,
		WarehouseID:  1,
		Quantity:     10,
		ReservedQty:  8,
		AvailableQty: 2,
	})
	uc := usecaseStock.NewStockUsecaseWithTx(repo, inlineTxMgr{})

	_, err := uc.AdjustStock(context.Background(), usecaseStock.AdjustInput{
		SKUID:       1,
		WarehouseID: 1,
		Type:        domainStock.MovementAdjust,
		Quantity:    5, // below reserved=8; old code produced available=-3
	})

	assert.Error(t, err)
	appErr, ok := err.(*appErrors.AppError)
	assert.True(t, ok)
	assert.Equal(t, "BELOW_RESERVED", appErr.Code)
	// Stock must be untouched
	assert.Equal(t, 10, repo.stocks[1].Quantity)
	assert.Equal(t, 0, len(repo.movements))
}

// FULL-08: adjustment and movement creation run in the same transaction;
// a failed movement write rolls back the quantity change.
func TestAdjustStock_MovementFailureRollsBack(t *testing.T) {
	repo := newFakeStockRepo(&domainStock.Stock{
		SKUID:       1,
		WarehouseID: 1,
		Quantity:    10,
	})
	repo.failMovement = true
	uc := usecaseStock.NewStockUsecaseWithTx(repo, rollbackTxMgr{repo: repo})

	_, err := uc.AdjustStock(context.Background(), usecaseStock.AdjustInput{
		SKUID:       1,
		WarehouseID: 1,
		Type:        domainStock.MovementAdjust,
		Quantity:    3,
	})

	assert.Error(t, err)
	// Quantity change must be rolled back to the original value
	assert.Equal(t, 10, repo.stocks[1].Quantity)
}

// FULL-08: OUT movements cannot oversell past availability.
func TestAdjustStock_RejectsOutBeyondAvailable(t *testing.T) {
	repo := newFakeStockRepo(&domainStock.Stock{
		SKUID:       1,
		WarehouseID: 1,
		Quantity:    10,
		ReservedQty: 8,
	})
	uc := usecaseStock.NewStockUsecaseWithTx(repo, inlineTxMgr{})

	_, err := uc.AdjustStock(context.Background(), usecaseStock.AdjustInput{
		SKUID:       1,
		WarehouseID: 1,
		Type:        domainStock.MovementOut,
		Quantity:    5, // available is only 2
	})
	assert.Error(t, err)
	assert.Equal(t, 10, repo.stocks[1].Quantity)
}

// Sanity: a valid ADJUST to a value at/above reserved still works.
func TestAdjustStock_ValidAdjustSucceeds(t *testing.T) {
	repo := newFakeStockRepo(&domainStock.Stock{
		SKUID:        1,
		WarehouseID:  1,
		Quantity:     10,
		ReservedQty:  8,
		AvailableQty: 2,
	})
	uc := usecaseStock.NewStockUsecaseWithTx(repo, inlineTxMgr{})

	updated, err := uc.AdjustStock(context.Background(), usecaseStock.AdjustInput{
		SKUID:       1,
		WarehouseID: 1,
		Type:        domainStock.MovementAdjust,
		Quantity:    8, // equal to reserved is allowed
	})
	assert.NoError(t, err)
	assert.Equal(t, 8, updated.Quantity)
	assert.Equal(t, 0, updated.AvailableQty)
	assert.Equal(t, 1, len(repo.movements))
}
