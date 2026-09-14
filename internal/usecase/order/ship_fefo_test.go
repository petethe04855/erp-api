package order_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	domainOrder "chawy-erp-api/internal/domain/order"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecaseOrder "chawy-erp-api/internal/usecase/order"
)

type fefoMockStockRepo struct {
	stocks    map[string]*domainStock.Stock
	lots      map[uint][]*domainStock.StockLot
	movements []*domainStock.StockMovement
}

func newFefoMockStockRepo() *fefoMockStockRepo {
	return &fefoMockStockRepo{
		stocks: make(map[string]*domainStock.Stock),
		lots:   make(map[uint][]*domainStock.StockLot),
	}
}

func (m *fefoMockStockRepo) key(skuID, whID uint) string {
	return fmt.Sprintf("%d-%d", skuID, whID)
}

func (m *fefoMockStockRepo) GetBySKUID(ctx context.Context, skuID, whID uint) (*domainStock.Stock, error) {
	s, ok := m.stocks[m.key(skuID, whID)]
	if !ok {
		return nil, nil
	}
	cp := *s
	return &cp, nil
}

func (m *fefoMockStockRepo) GetBySKUIDForUpdate(ctx context.Context, skuID, whID uint) (*domainStock.Stock, error) {
	return m.GetBySKUID(ctx, skuID, whID)
}

func (m *fefoMockStockRepo) FindAll(ctx context.Context, query domainStock.Query) ([]domainStock.Stock, int64, error) {
	return nil, 0, nil
}

func (m *fefoMockStockRepo) FindAllBySKU(ctx context.Context, query domainStock.StockBySKUQuery) ([]domainStock.StockBySKU, int64, error) {
	return nil, 0, nil
}

func (m *fefoMockStockRepo) UpdateQuantity(ctx context.Context, skuID, whID uint, delta int) (*domainStock.Stock, error) {
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

func (m *fefoMockStockRepo) ReserveStock(ctx context.Context, skuID, whID uint, qty int) (*domainStock.Stock, error) {
	return nil, nil
}

func (m *fefoMockStockRepo) ReleaseStock(ctx context.Context, skuID, whID uint, qty int) (*domainStock.Stock, error) {
	return nil, nil
}

func (m *fefoMockStockRepo) CreateMovement(ctx context.Context, movement *domainStock.StockMovement) error {
	m.movements = append(m.movements, movement)
	return nil
}

func (m *fefoMockStockRepo) GetMovements(ctx context.Context, skuID uint, page, limit int) ([]domainStock.StockMovement, int64, error) {
	return nil, 0, nil
}

func (m *fefoMockStockRepo) CreateLot(ctx context.Context, lot *domainStock.StockLot) error {
	lot.ID = uint(len(m.lots[lot.SKUID]) + 1)
	m.lots[lot.SKUID] = append(m.lots[lot.SKUID], lot)
	return nil
}

func (m *fefoMockStockRepo) GetAvailableLotsForUpdate(ctx context.Context, skuID, whID uint) ([]domainStock.StockLot, error) {
	lots := m.lots[skuID]
	var res []domainStock.StockLot
	for _, l := range lots {
		if (l.Quantity - l.ReservedQty) > 0 {
			res = append(res, *l)
		}
	}
	return res, nil
}

func (m *fefoMockStockRepo) DeductLotQuantity(ctx context.Context, lotID uint, qty int) (*domainStock.StockLot, error) {
	for _, lotList := range m.lots {
		for _, l := range lotList {
			if l.ID == lotID {
				l.Quantity -= qty
				l.AvailableQty = l.Quantity - l.ReservedQty
				cp := *l
				return &cp, nil
			}
		}
	}
	return nil, fmt.Errorf("lot %d not found", lotID)
}

func (m *fefoMockStockRepo) FindLotsBySKU(ctx context.Context, skuID, whID uint) ([]domainStock.StockLot, error) {
	return nil, nil
}

func TestOrderShip_FEFOAllocation(t *testing.T) {
	orderRepo := newMockOrderRepo()
	skuRepo := newMockSKURepo()
	bundleRepo := newMockBundleRepo()
	formulaRepo := newMockFormulaRepo()
	stockRepo := newFefoMockStockRepo()
	txMgr := &fakeTxManager{}

	skuRepo.skus["SKU-FEFO"] = &domainSKU.SKU{ID: 101, SKU: "SKU-FEFO", Name: "Fresh Produce", Price: 200}
	stockRepo.stocks["101-1"] = &domainStock.Stock{ID: 1, SKUID: 101, WarehouseID: 1, Quantity: 15, AvailableQty: 15}

	// Create 2 lots: Lot 1 expires earlier (2026-09-01), Lot 2 expires later (2026-10-01)
	lot1 := &domainStock.StockLot{
		ID:           1,
		SKUID:        101,
		WarehouseID:  1,
		LotNumber:    "LOT-EARLY",
		ExpiryDate:   "2026-09-01",
		Quantity:     6,
		AvailableQty: 6,
		ReceivedAt:   time.Now().Add(-48 * time.Hour),
	}
	lot2 := &domainStock.StockLot{
		ID:           2,
		SKUID:        101,
		WarehouseID:  1,
		LotNumber:    "LOT-LATER",
		ExpiryDate:   "2026-10-01",
		Quantity:     9,
		AvailableQty: 9,
		ReceivedAt:   time.Now().Add(-24 * time.Hour),
	}
	stockRepo.lots[101] = []*domainStock.StockLot{lot1, lot2}

	order := &domainOrder.Order{
		ID:      1,
		OrderNo: "SO-FEFO-01",
		Status:  domainOrder.StatusPending,
		Items: []domainOrder.OrderItem{
			{SKU: "SKU-FEFO", Quantity: 8, Price: 200},
		},
	}
	orderRepo.orders[1] = order

	uc := usecaseOrder.NewOrderUsecaseWithTx(nil, orderRepo, skuRepo, bundleRepo, formulaRepo, stockRepo, txMgr)

	shippedOrder, err := uc.ShipOrder(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("ShipOrder failed: %v", err)
	}

	if shippedOrder.Status != domainOrder.StatusShipped {
		t.Errorf("expected status SHIPPED, got %s", shippedOrder.Status)
	}

	// Check that Lot 1 (early) was completely consumed (6 deducted, 0 remaining)
	if lot1.Quantity != 0 {
		t.Errorf("expected lot1 quantity 0, got %d", lot1.Quantity)
	}
	// Check that Lot 2 (later) provided the remaining 2 units (9 - 2 = 7 remaining)
	if lot2.Quantity != 7 {
		t.Errorf("expected lot2 quantity 7, got %d", lot2.Quantity)
	}

	// Verify StockMovements were recorded with Lot metadata
	if len(stockRepo.movements) != 2 {
		t.Fatalf("expected 2 movements (one per lot), got %d", len(stockRepo.movements))
	}

	firstMov := stockRepo.movements[0]
	if *firstMov.StockLotID != lot1.ID || firstMov.Quantity != 6 {
		t.Errorf("first movement should be Lot 1 x 6, got Lot %v x %d", *firstMov.StockLotID, firstMov.Quantity)
	}

	secondMov := stockRepo.movements[1]
	if *secondMov.StockLotID != lot2.ID || secondMov.Quantity != 2 {
		t.Errorf("second movement should be Lot 2 x 2, got Lot %v x %d", *secondMov.StockLotID, secondMov.Quantity)
	}
}
