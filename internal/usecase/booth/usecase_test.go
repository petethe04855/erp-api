package booth

import (
	"context"
	"errors"
	"testing"
	"time"

	domainBooth "chawy-erp-api/internal/domain/booth"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/require"
)

// ---- fakes ----

type fakeRepo struct {
	cfg   *domainBooth.ConfigRecord
	sales []domainBooth.Sale
}

func (f *fakeRepo) GetConfig(context.Context) (*domainBooth.ConfigRecord, error) { return f.cfg, nil }
func (f *fakeRepo) SaveConfig(_ context.Context, r *domainBooth.ConfigRecord) error {
	cp := *r
	f.cfg = &cp
	return nil
}
func (f *fakeRepo) FindByClientRef(_ context.Context, ref string) (*domainBooth.Sale, error) {
	for i := range f.sales {
		if f.sales[i].ClientRef == ref {
			cp := f.sales[i]
			return &cp, nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) FindByIDForUpdate(_ context.Context, id uint) (*domainBooth.Sale, error) {
	for i := range f.sales {
		if f.sales[i].ID == id {
			cp := f.sales[i]
			return &cp, nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) Create(_ context.Context, s *domainBooth.Sale) error {
	s.ID = uint(len(f.sales) + 1)
	f.sales = append(f.sales, *s)
	return nil
}
func (f *fakeRepo) SetSaleNo(_ context.Context, id uint, no string) error {
	f.sales[id-1].SaleNo = no
	return nil
}
func (f *fakeRepo) MarkVoided(_ context.Context, id uint, by, reason string, at time.Time) error {
	f.sales[id-1].Status, f.sales[id-1].VoidedBy, f.sales[id-1].VoidReason, f.sales[id-1].VoidedAt = domainBooth.StatusVoided, by, reason, &at
	return nil
}
func (f *fakeRepo) List(_ context.Context, flt domainBooth.SaleFilter) ([]domainBooth.Sale, error) {
	var out []domainBooth.Sale
	for _, s := range f.sales {
		if !s.CreatedAt.Before(flt.From) && s.CreatedAt.Before(flt.To) {
			out = append(out, s)
		}
	}
	return out, nil
}

type fakeSKUs struct{ items map[uint]domainSKU.SKU }

func (f *fakeSKUs) FindByID(_ context.Context, id uint) (*domainSKU.SKU, error) {
	if s, ok := f.items[id]; ok {
		return &s, nil
	}
	return nil, nil
}

type fakeStock struct {
	rows      map[uint]*domainStock.Stock
	lots      map[uint]*domainStock.StockLot
	movements []domainStock.StockMovement
}

func (f *fakeStock) GetBySKUID(_ context.Context, sku, _ uint) (*domainStock.Stock, error) {
	if s, ok := f.rows[sku]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}
func (f *fakeStock) GetBySKUIDForUpdate(ctx context.Context, sku, wh uint) (*domainStock.Stock, error) {
	return f.GetBySKUID(ctx, sku, wh)
}
func (f *fakeStock) UpdateQuantity(_ context.Context, sku, _ uint, delta int) (*domainStock.Stock, error) {
	s := f.rows[sku]
	s.Quantity += delta
	s.AvailableQty = s.Quantity - s.ReservedQty
	cp := *s
	return &cp, nil
}
func (f *fakeStock) CreateMovement(_ context.Context, m *domainStock.StockMovement) error {
	f.movements = append(f.movements, *m)
	return nil
}
func (f *fakeStock) GetAvailableLotsForUpdate(_ context.Context, sku, _ uint) ([]domainStock.StockLot, error) {
	var out []domainStock.StockLot
	for id := uint(1); id <= uint(len(f.lots)); id++ {
		if l := f.lots[id]; l != nil && l.SKUID == sku && l.Quantity-l.ReservedQty > 0 {
			out = append(out, *l)
		}
	}
	return out, nil
}
func (f *fakeStock) DeductLotQuantity(_ context.Context, id uint, qty int) (*domainStock.StockLot, error) {
	l := f.lots[id]
	l.Quantity -= qty
	cp := *l
	return &cp, nil
}

type fakeTx struct{}

func (fakeTx) Transaction(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// ---- setup ----

const (
	skuChicken = 11
	skuTuna    = 12
	skuWand    = 21
)

func newTestUC() (*usecase, *fakeRepo, *fakeStock) {
	cfg := DefaultConfig()
	cfg.Flavors = []domainBooth.Flavor{{SKUID: skuChicken, Name: "ไก่", Color: "#D71920"}, {SKUID: skuTuna, Name: "ทูน่า", Color: "#0047B8"}}
	cfg.Items = []domainBooth.Item{{SKUID: skuWand, Name: "ไม้ตกแมว", Price: 199}}
	repo := &fakeRepo{cfg: &domainBooth.ConfigRecord{ID: 1, Data: cfg}}
	skus := &fakeSKUs{items: map[uint]domainSKU.SKU{
		skuChicken: {ID: skuChicken, SKU: "TRT-CHK"}, skuTuna: {ID: skuTuna, SKU: "TRT-TUN"}, skuWand: {ID: skuWand, SKU: "TOY-WAND"},
	}}
	stock := &fakeStock{
		rows: map[uint]*domainStock.Stock{
			skuChicken: {SKUID: skuChicken, Quantity: 10, AvailableQty: 10},
			skuTuna:    {SKUID: skuTuna, Quantity: 2, AvailableQty: 2},
			skuWand:    {SKUID: skuWand, Quantity: 5, ReservedQty: 4, AvailableQty: 1},
		},
		lots: map[uint]*domainStock.StockLot{
			1: {ID: 1, SKUID: skuChicken, LotNumber: "L-OLD", Quantity: 3},
			2: {ID: 2, SKUID: skuChicken, LotNumber: "L-NEW", Quantity: 7},
		},
	}
	uc := New(repo, skus, stock, fakeTx{}).(*usecase)
	uc.now = func() time.Time { return time.Date(2026, 11, 26, 4, 0, 0, 0, time.UTC) } // 11:00 Bangkok
	return uc, repo, stock
}

func f64(v float64) *float64 { return &v }

// ---- pricing ----

func TestBestSetBreakdownPicksCheapest(t *testing.T) {
	sets := DefaultConfig().Sets
	cases := []struct {
		bags  int
		total float64
		label string
	}{
		{1, 129, "เซต 1"}, {2, 258, "เซต 1 ×2"}, {3, 289, "เซต 3"}, {4, 418, "เซต 3 + เซต 1"},
		{6, 578, "เซต 3 ×2"}, {8, 748, "เซต 5 + เซต 3"}, {13, 1179, "เซต 10 + เซต 3"},
	}
	for _, c := range cases {
		parts, total, err := BestSetBreakdown(c.bags, sets)
		require.NoError(t, err)
		require.Equal(t, c.total, total, "bags %d", c.bags)
		require.Equal(t, c.label, SetLabel(parts), "bags %d", c.bags)
	}
	_, _, err := BestSetBreakdown(2, []domainBooth.SetPrice{{Size: 3, Price: 289}})
	require.Error(t, err)
}

func TestPriceCartDiscountAndShares(t *testing.T) {
	uc, _, _ := newTestUC()
	cfg, _, _ := uc.loadConfig(context.Background())
	// 5 bags (459) + 3 wands (597) = 1056 >= 1000 → 50 off
	bill, err := PriceCart(cfg, map[uint]int{skuChicken: 3, skuTuna: 2, skuWand: 3})
	require.NoError(t, err)
	require.Equal(t, 5, bill.Bags)
	require.Equal(t, 459.0, bill.TreatTotal)
	require.Equal(t, 1056.0, bill.Subtotal)
	require.Equal(t, 50.0, bill.Discount)
	require.Equal(t, 1006.0, bill.Total)
	require.Equal(t, 275.4, bill.Lines[0].Amount) // 459 × 3/5
	require.Equal(t, 183.6, bill.Lines[1].Amount)

	_, err = PriceCart(cfg, map[uint]int{999: 1})
	require.Error(t, err)
}

// ---- sales ----

func TestCreateSaleDeductsInventoryFEFO(t *testing.T) {
	uc, repo, stock := newTestUC()
	res, err := uc.CreateSale(context.Background(), SaleInput{
		ClientRef: "dev1-0001", Lines: []CartLine{{SKUID: skuChicken, Qty: 4}, {SKUID: skuTuna, Qty: 1}},
		PaymentMethod: domainBooth.PayCash, CashReceived: f64(500), ExpectedTotal: f64(459), SellerID: 7, SellerName: "a@chawy",
	})
	require.NoError(t, err)
	require.False(t, res.Replayed)
	require.Equal(t, "BTH-000001", res.Sale.SaleNo)
	require.Equal(t, 459.0, res.Sale.Total) // 5 bags = เซต 5
	require.Equal(t, 41.0, res.Sale.Change)

	require.Equal(t, 6, stock.rows[skuChicken].Quantity)
	require.Equal(t, 1, stock.rows[skuTuna].Quantity)
	require.Equal(t, 0, stock.lots[1].Quantity) // oldest lot first
	require.Equal(t, 6, stock.lots[2].Quantity)

	// chicken: two lot movements, tuna: one lot-less movement
	require.Len(t, stock.movements, 3)
	for _, m := range stock.movements {
		require.Equal(t, domainStock.MovementOut, m.Type)
		require.Equal(t, "BOOTH", m.Channel)
		require.Equal(t, "BOOTH_SALE", m.ReferenceType)
		require.Equal(t, "BTH-000001", m.ReferenceID)
	}
	require.Equal(t, []domainBooth.LotAlloc{{LotID: 1, LotNumber: "L-OLD", Qty: 3}, {LotID: 2, LotNumber: "L-NEW", Qty: 1}}, repo.sales[0].Lines[0].Lots)
}

func TestCreateSaleReplayDeductsOnce(t *testing.T) {
	uc, repo, stock := newTestUC()
	in := SaleInput{ClientRef: "dev1-0002", Lines: []CartLine{{SKUID: skuChicken, Qty: 1}}, PaymentMethod: domainBooth.PayTransfer}
	_, err := uc.CreateSale(context.Background(), in)
	require.NoError(t, err)
	res, err := uc.CreateSale(context.Background(), in)
	require.NoError(t, err)
	require.True(t, res.Replayed)
	require.Len(t, repo.sales, 1)
	require.Equal(t, 9, stock.rows[skuChicken].Quantity)
}

func TestCreateSaleShortReportsAllAndDeductsNothing(t *testing.T) {
	uc, repo, stock := newTestUC()
	_, err := uc.CreateSale(context.Background(), SaleInput{
		ClientRef: "dev1-0003", PaymentMethod: domainBooth.PayCard,
		Lines: []CartLine{{SKUID: skuTuna, Qty: 3}, {SKUID: skuWand, Qty: 2}, {SKUID: skuChicken, Qty: 1}},
	})
	var short *InsufficientStockError
	require.True(t, errors.As(err, &short))
	require.Len(t, short.Shortages, 2) // tuna 2 left, wand only 1 available (4 reserved)
	require.True(t, errors.Is(err, appErrors.ErrInsufficientStock))
	require.Empty(t, repo.sales)
	require.Empty(t, stock.movements)
	require.Equal(t, 10, stock.rows[skuChicken].Quantity)
}

func TestCreateSaleRefusesStalePriceAndShortCash(t *testing.T) {
	uc, repo, _ := newTestUC()
	_, err := uc.CreateSale(context.Background(), SaleInput{
		ClientRef: "dev1-0004", Lines: []CartLine{{SKUID: skuChicken, Qty: 1}}, PaymentMethod: domainBooth.PayCash, ExpectedTotal: f64(120),
	})
	var changed *PriceChangedError
	require.True(t, errors.As(err, &changed))
	require.Equal(t, 129.0, changed.Bill.Total)

	_, err = uc.CreateSale(context.Background(), SaleInput{
		ClientRef: "dev1-0005", Lines: []CartLine{{SKUID: skuChicken, Qty: 1}}, PaymentMethod: domainBooth.PayCash, CashReceived: f64(100),
	})
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "CASH_SHORT", appErr.Code)
	require.Empty(t, repo.sales)
}

func TestVoidSaleReturnsStockToSameLots(t *testing.T) {
	uc, repo, stock := newTestUC()
	res, err := uc.CreateSale(context.Background(), SaleInput{
		ClientRef: "dev1-0006", Lines: []CartLine{{SKUID: skuChicken, Qty: 4}}, PaymentMethod: domainBooth.PayCard,
	})
	require.NoError(t, err)

	sale, err := uc.VoidSale(context.Background(), res.Sale.ID, "owner@chawy", "คีย์ผิด")
	require.NoError(t, err)
	require.Equal(t, domainBooth.StatusVoided, sale.Status)
	require.Equal(t, domainBooth.StatusVoided, repo.sales[0].Status)
	require.Equal(t, 10, stock.rows[skuChicken].Quantity)
	require.Equal(t, 3, stock.lots[1].Quantity)
	require.Equal(t, 7, stock.lots[2].Quantity)
	last := stock.movements[len(stock.movements)-1]
	require.Equal(t, domainStock.MovementIn, last.Type)
	require.Equal(t, "BOOTH_VOID", last.ReferenceType)
	require.Equal(t, 4, last.Quantity)

	_, err = uc.VoidSale(context.Background(), res.Sale.ID, "owner@chawy", "")
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "ALREADY_VOIDED", appErr.Code)
	require.Equal(t, 10, stock.rows[skuChicken].Quantity)
}

func TestListSalesUsesBangkokDay(t *testing.T) {
	uc, _, _ := newTestUC()
	_, err := uc.CreateSale(context.Background(), SaleInput{ClientRef: "a", Lines: []CartLine{{SKUID: skuChicken, Qty: 1}}, PaymentMethod: domainBooth.PayCard})
	require.NoError(t, err)
	today, err := uc.ListSales(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, today, 1)
	other, err := uc.ListSales(context.Background(), "2026-11-25")
	require.NoError(t, err)
	require.Empty(t, other)
	_, err = uc.ListSales(context.Background(), "26/11/2026")
	require.Error(t, err)
}

func TestUpdateConfigValidates(t *testing.T) {
	uc, _, _ := newTestUC()
	cfg := DefaultConfig()
	cfg.Flavors = []domainBooth.Flavor{{SKUID: skuChicken, Name: "ไก่", Color: "#D71920"}}
	cfg.Items = []domainBooth.Item{{SKUID: skuChicken, Name: "ซ้ำ", Price: 10}}
	_, err := uc.UpdateConfig(context.Background(), cfg, "owner")
	require.Error(t, err) // same SKU on two buttons

	cfg.Items = []domainBooth.Item{{SKUID: 404, Name: "ไม่มี", Price: 10}}
	_, err = uc.UpdateConfig(context.Background(), cfg, "owner")
	require.Error(t, err)

	cfg.Items = []domainBooth.Item{{SKUID: skuWand, Name: "ไม้ตกแมว", Price: 199}}
	cat, err := uc.UpdateConfig(context.Background(), cfg, "owner")
	require.NoError(t, err)
	require.Len(t, cat.Buttons, 2)
	require.Equal(t, 10, cat.Buttons[0].Available)
	require.Equal(t, 1, cat.Buttons[1].Available)
	require.Equal(t, "TOY-WAND", cat.Buttons[1].SKUCode)
}
