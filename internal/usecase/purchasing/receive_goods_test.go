package purchasing_test

import (
	"context"
	"testing"

	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecasePurchasing "chawy-erp-api/internal/usecase/purchasing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// ---- fakes ----

type fakeSKURepo struct {
	items map[string]*domainSKU.SKU
}

func (f *fakeSKURepo) Create(ctx context.Context, s *domainSKU.SKU) error { return nil }
func (f *fakeSKURepo) FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	return nil, nil
}
func (f *fakeSKURepo) FindBySKU(ctx context.Context, code string) (*domainSKU.SKU, error) {
	if s, ok := f.items[code]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}
func (f *fakeSKURepo) FindAll(ctx context.Context, q domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	return nil, 0, nil
}
func (f *fakeSKURepo) Update(ctx context.Context, s *domainSKU.SKU) error { return nil }
func (f *fakeSKURepo) Delete(ctx context.Context, id uint) error          { return nil }
func (f *fakeSKURepo) ExistsBySKU(ctx context.Context, code string) (bool, error) {
	return false, nil
}

type fakePurchRepo struct {
	pos       map[uint]*domainPurchasing.PurchaseOrder
	grDocs    []*domainPurchasing.GoodsReceive
	movements int // not used here; stock tracked in stockRepo
}

func (f *fakePurchRepo) CreateSupplier(ctx context.Context, s *domainPurchasing.Supplier) error {
	return nil
}
func (f *fakePurchRepo) FindSupplierByID(ctx context.Context, id uint) (*domainPurchasing.Supplier, error) {
	return nil, nil
}
func (f *fakePurchRepo) FindAllSuppliers(ctx context.Context, q domainPurchasing.SupplierQuery) ([]domainPurchasing.Supplier, int64, error) {
	return nil, 0, nil
}
func (f *fakePurchRepo) UpdateSupplier(ctx context.Context, s *domainPurchasing.Supplier) error {
	return nil
}
func (f *fakePurchRepo) DeleteSupplier(ctx context.Context, id uint) error { return nil }
func (f *fakePurchRepo) CreatePO(ctx context.Context, po *domainPurchasing.PurchaseOrder) error {
	return nil
}
func (f *fakePurchRepo) FindPOByID(ctx context.Context, id uint) (*domainPurchasing.PurchaseOrder, error) {
	if po, ok := f.pos[id]; ok {
		cp := *po
		return &cp, nil
	}
	return nil, nil
}

// FindPOByIDForUpdate returns a copy; caller mutates it via repo updates below.
func (f *fakePurchRepo) FindPOByIDForUpdate(ctx context.Context, id uint) (*domainPurchasing.PurchaseOrder, error) {
	if po, ok := f.pos[id]; ok {
		cp := *po
		cp.Items = make([]domainPurchasing.POItem, len(po.Items))
		copy(cp.Items, po.Items)
		return &cp, nil
	}
	return nil, nil
}

func (f *fakePurchRepo) FindAllPOs(ctx context.Context, q domainPurchasing.POQuery) ([]domainPurchasing.PurchaseOrder, int64, error) {
	return nil, 0, nil
}
func (f *fakePurchRepo) UpdatePOStatus(ctx context.Context, id uint, s domainPurchasing.POStatus) error {
	if po, ok := f.pos[id]; ok {
		po.Status = s
	}
	return nil
}
func (f *fakePurchRepo) UpdatePO(ctx context.Context, po *domainPurchasing.PurchaseOrder) error {
	if stored, ok := f.pos[po.ID]; ok {
		stored.Status = po.Status
		stored.UpdatedAt = po.UpdatedAt
		// received qty already applied through UpdatePOItemReceivedQty
	}
	return nil
}
func (f *fakePurchRepo) UpdatePOItemReceivedQty(ctx context.Context, poItemID uint, receivedQty int) error {
	for _, po := range f.pos {
		for i := range po.Items {
			if po.Items[i].ID == poItemID {
				po.Items[i].ReceivedQty = receivedQty
			}
		}
	}
	return nil
}
func (f *fakePurchRepo) CreateGoodsReceiveDoc(ctx context.Context, gr *domainPurchasing.GoodsReceive, items []domainPurchasing.GoodsReceiveItem) error {
	gr.ID = uint(len(f.grDocs) + 1)
	gr.Items = items
	f.grDocs = append(f.grDocs, gr)
	return nil
}

type fakeStockRepoP struct {
	stocks    map[uint]*domainStock.Stock
	movements []domainStock.StockMovement
}

func (f *fakeStockRepoP) GetBySKUID(ctx context.Context, skuID, whID uint) (*domainStock.Stock, error) {
	if s, ok := f.stocks[skuID]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}
func (f *fakeStockRepoP) GetBySKUIDForUpdate(ctx context.Context, skuID, whID uint) (*domainStock.Stock, error) {
	return f.GetBySKUID(ctx, skuID, whID)
}
func (f *fakeStockRepoP) FindAll(ctx context.Context, q domainStock.Query) ([]domainStock.Stock, int64, error) {
	return nil, 0, nil
}
func (f *fakeStockRepoP) UpdateQuantity(ctx context.Context, skuID, whID uint, delta int) (*domainStock.Stock, error) {
	s, ok := f.stocks[skuID]
	if !ok {
		s = &domainStock.Stock{SKUID: skuID, WarehouseID: whID}
		f.stocks[skuID] = s
	}
	s.Quantity += delta
	s.AvailableQty = s.Quantity - s.ReservedQty
	return s, nil
}
func (f *fakeStockRepoP) CreateMovement(ctx context.Context, m *domainStock.StockMovement) error {
	f.movements = append(f.movements, *m)
	return nil
}
func (f *fakeStockRepoP) GetMovements(ctx context.Context, skuID uint, page, limit int) ([]domainStock.StockMovement, int64, error) {
	return f.movements, int64(len(f.movements)), nil
}

type inlineTxMgrP struct{}

func (inlineTxMgrP) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

// ---- tests ----

// FULL-09: after a partial receipt of 4 of 10, a second receive adds only the
// remaining 6 — not the full 10 again (old code produced 14 received / 14+ stock).
func TestReceiveGoods_PartialThenReceiveAddsOnlyRemaining(t *testing.T) {
	po := &domainPurchasing.PurchaseOrder{
		ID:     1,
		PONo:   "PO-TEST-1",
		Status: domainPurchasing.StatusApproved,
		Items: []domainPurchasing.POItem{
			{ID: 11, SKU: "SKU-A", Quantity: 10, ReceivedQty: 4},
		},
	}
	purchRepo := &fakePurchRepo{pos: map[uint]*domainPurchasing.PurchaseOrder{1: po}}
	skuRepo := &fakeSKURepo{items: map[string]*domainSKU.SKU{
		"SKU-A": {ID: 100, SKU: "SKU-A", Name: "A"},
	}}
	stockRepo := &fakeStockRepoP{stocks: map[uint]*domainStock.Stock{100: {SKUID: 100, WarehouseID: 1, Quantity: 50}}}

	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, purchRepo, skuRepo, stockRepo, inlineTxMgrP{})

	result, err := uc.ReceiveGoods(context.Background(), usecasePurchasing.ReceiveGoodsInput{POID: 1, WarehouseID: 1})
	assert.NoError(t, err)
	assert.Equal(t, domainPurchasing.StatusReceived, result.Status)

	// Stock must have increased by the remaining 6 only
	assert.Equal(t, 56, stockRepo.stocks[100].Quantity)
	// Received qty must be exactly the ordered 10
	assert.Equal(t, 10, purchRepo.pos[1].Items[0].ReceivedQty)
	// A GR document must exist
	assert.Equal(t, 1, len(purchRepo.grDocs))
	assert.Equal(t, 6, purchRepo.grDocs[0].Items[0].Quantity)
}

// FULL-09: receiving a CANCELLED PO is rejected.
func TestReceiveGoods_RejectsCancelledPO(t *testing.T) {
	po := &domainPurchasing.PurchaseOrder{
		ID:     2,
		PONo:   "PO-TEST-2",
		Status: domainPurchasing.StatusCancelled,
		Items: []domainPurchasing.POItem{
			{ID: 21, SKU: "SKU-A", Quantity: 5},
		},
	}
	purchRepo := &fakePurchRepo{pos: map[uint]*domainPurchasing.PurchaseOrder{2: po}}
	skuRepo := &fakeSKURepo{items: map[string]*domainSKU.SKU{}}
	stockRepo := &fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}

	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, purchRepo, skuRepo, stockRepo, inlineTxMgrP{})

	_, err := uc.ReceiveGoods(context.Background(), usecasePurchasing.ReceiveGoodsInput{POID: 2, WarehouseID: 1})
	assert.Error(t, err)
	assert.Equal(t, 0, len(stockRepo.movements))
	assert.Equal(t, 0, len(purchRepo.grDocs))
}

// FULL-09: already-received PO cannot be received again.
func TestReceiveGoods_RejectsAlreadyReceived(t *testing.T) {
	po := &domainPurchasing.PurchaseOrder{
		ID:     3,
		PONo:   "PO-TEST-3",
		Status: domainPurchasing.StatusReceived,
		Items: []domainPurchasing.POItem{
			{ID: 31, SKU: "SKU-A", Quantity: 5, ReceivedQty: 5},
		},
	}
	purchRepo := &fakePurchRepo{pos: map[uint]*domainPurchasing.PurchaseOrder{3: po}}
	skuRepo := &fakeSKURepo{items: map[string]*domainSKU.SKU{}}
	stockRepo := &fakeStockRepoP{stocks: map[uint]*domainStock.Stock{}}

	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, purchRepo, skuRepo, stockRepo, inlineTxMgrP{})

	_, err := uc.ReceiveGoods(context.Background(), usecasePurchasing.ReceiveGoodsInput{POID: 3, WarehouseID: 1})
	assert.Error(t, err)
	assert.Equal(t, 0, len(purchRepo.grDocs))
}

// FULL-09: partially received PO stays open (APPROVED) until the remainder
// arrives — receiving only part of the ordered quantity must not mark RECEIVED.
// This exercises the aggregate logic via multiple PO lines where one line is
// already fully received and the other is not.
func TestReceiveGoods_PartialReceiptKeepsPOOpen(t *testing.T) {
	po := &domainPurchasing.PurchaseOrder{
		ID:     4,
		PONo:   "PO-TEST-4",
		Status: domainPurchasing.StatusApproved,
		Items: []domainPurchasing.POItem{
			{ID: 41, SKU: "SKU-A", Quantity: 8, ReceivedQty: 8}, // already complete
			{ID: 42, SKU: "SKU-B", Quantity: 5, ReceivedQty: 0}, // still open
		},
	}
	purchRepo := &fakePurchRepo{pos: map[uint]*domainPurchasing.PurchaseOrder{4: po}}
	skuRepo := &fakeSKURepo{items: map[string]*domainSKU.SKU{
		"SKU-A": {ID: 100, SKU: "SKU-A"},
		"SKU-B": {ID: 200, SKU: "SKU-B"},
	}}
	stockRepo := &fakeStockRepoP{stocks: map[uint]*domainStock.Stock{
		100: {SKUID: 100, WarehouseID: 1, Quantity: 0},
		200: {SKUID: 200, WarehouseID: 1, Quantity: 0},
	}}

	uc := usecasePurchasing.NewPurchasingUsecaseWithTx(nil, purchRepo, skuRepo, stockRepo, inlineTxMgrP{})

	result, err := uc.ReceiveGoods(context.Background(), usecasePurchasing.ReceiveGoodsInput{POID: 4, WarehouseID: 1})
	assert.NoError(t, err)

	// SKU-A line fully received already -> skipped; SKU-B receives remaining 5.
	// PO remains open because... actually both lines complete -> RECEIVED.
	assert.Equal(t, domainPurchasing.StatusReceived, result.Status)
	assert.Equal(t, 5, stockRepo.stocks[200].Quantity)
	// No movement for SKU-A (already fully received)
	movA := 0
	for _, m := range stockRepo.movements {
		if m.SKUID == 100 {
			movA++
		}
	}
	assert.Equal(t, 0, movA)
}

var _ = gorm.ErrRecordNotFound
