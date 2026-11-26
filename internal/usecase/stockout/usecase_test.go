package stockout

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainStockOut "chawy-erp-api/internal/domain/stockout"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/require"
)

// ---- fakes ----

type fakeRepo struct {
	cfg     *domainStockOut.ConfigRecord
	batches []domainStockOut.Batch
}

func (f *fakeRepo) GetConfig(context.Context) (*domainStockOut.ConfigRecord, error) {
	return f.cfg, nil
}
func (f *fakeRepo) SaveConfig(_ context.Context, r *domainStockOut.ConfigRecord) error {
	cp := *r
	f.cfg = &cp
	return nil
}
func (f *fakeRepo) FindByClientID(_ context.Context, id string) (*domainStockOut.Batch, error) {
	for i := range f.batches {
		if f.batches[i].ClientID == id {
			cp := f.batches[i]
			return &cp, nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) FindByIDForUpdate(_ context.Context, id uint) (*domainStockOut.Batch, error) {
	if id == 0 || int(id) > len(f.batches) {
		return nil, nil
	}
	cp := f.batches[id-1]
	return &cp, nil
}
func (f *fakeRepo) Create(_ context.Context, b *domainStockOut.Batch) error {
	b.ID = uint(len(f.batches) + 1)
	f.batches = append(f.batches, *b)
	return nil
}
func (f *fakeRepo) SetBatchNo(_ context.Context, id uint, no string) error {
	f.batches[id-1].BatchNo = no
	return nil
}
func (f *fakeRepo) MarkVoided(_ context.Context, id uint, by string, at time.Time) error {
	f.batches[id-1].Status, f.batches[id-1].VoidedBy, f.batches[id-1].VoidedAt = domainStockOut.StatusVoided, by, &at
	return nil
}
func (f *fakeRepo) MarkReviewed(_ context.Context, id uint, by, note string, at time.Time) error {
	f.batches[id-1].ReviewedBy, f.batches[id-1].ReviewNote, f.batches[id-1].ReviewedAt = by, note, &at
	return nil
}
func (f *fakeRepo) List(_ context.Context, flt domainStockOut.Filter) ([]domainStockOut.Batch, error) {
	var out []domainStockOut.Batch
	for _, b := range f.batches {
		if !b.CountedAt.Before(flt.From) && b.CountedAt.Before(flt.To) {
			out = append(out, b)
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
	lots      []*domainStock.StockLot
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
func (f *fakeStock) lotsFor(sku uint, freeOnly bool) []domainStock.StockLot {
	var out []domainStock.StockLot
	for _, l := range f.lots {
		if l.SKUID == sku && (!freeOnly || l.Quantity-l.ReservedQty > 0) {
			out = append(out, *l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiryDate < out[j].ExpiryDate })
	return out
}
func (f *fakeStock) GetAvailableLotsForUpdate(_ context.Context, sku, _ uint) ([]domainStock.StockLot, error) {
	return f.lotsFor(sku, true), nil
}
func (f *fakeStock) FindLotsBySKU(_ context.Context, sku, _ uint) ([]domainStock.StockLot, error) {
	return f.lotsFor(sku, false), nil
}
func (f *fakeStock) DeductLotQuantity(_ context.Context, id uint, qty int) (*domainStock.StockLot, error) {
	for _, l := range f.lots {
		if l.ID == id {
			l.Quantity -= qty
			cp := *l
			return &cp, nil
		}
	}
	return nil, errors.New("lot not found")
}

type fakeTx struct{}

func (fakeTx) Transaction(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// ---- setup ----

const (
	skuChicken = 11
	skuTuna    = 12
	skuLitter  = 31
	skuFish    = 41 // wand head "น้องปลา"
	skuBird    = 42 // wand head "นกน้อย"
)

var (
	packer = Actor{ID: 5, Role: "warehouse", Email: "pack@chawy"}
	other  = Actor{ID: 6, Role: "warehouse", Email: "pack2@chawy"}
	owner  = Actor{ID: 1, Role: "owner", Email: "owner@chawy"}
)

type env struct {
	uc    *usecase
	repo  *fakeRepo
	stock *fakeStock
	clock time.Time
}

func newEnv() *env {
	cfg := DefaultConfig()
	cfg.Boxes = []domainStockOut.Box{
		{SKUID: skuChicken, Name: "ไก่", Kind: "flavor", Unit: "ถุง", Color: "#D71920", LowStock: 20},
		{SKUID: skuTuna, Name: "ทูน่า", Kind: "flavor", Unit: "ถุง", Color: "#0047B8", LowStock: 20},
		{SKUID: skuLitter, Name: "ทรายแมว", Kind: "item", Unit: "ถุง"},
		{SKUID: skuFish, Name: "หัวเปลี่ยน น้องปลา", Kind: "item", Group: "หัวไม้ตกแมว", Unit: "ชิ้น"},
		{SKUID: skuBird, Name: "หัวเปลี่ยน นกน้อย", Kind: "item", Group: "หัวไม้ตกแมว", Unit: "ชิ้น"},
	}
	e := &env{repo: &fakeRepo{cfg: &domainStockOut.ConfigRecord{ID: 1, Data: cfg}}}
	skus := &fakeSKUs{items: map[uint]domainSKU.SKU{
		skuChicken: {ID: skuChicken, SKU: "TRT-CHK"}, skuTuna: {ID: skuTuna, SKU: "TRT-TUN"},
		skuLitter: {ID: skuLitter, SKU: "LIT-01"}, skuFish: {ID: skuFish, SKU: "WH-FISH"}, skuBird: {ID: skuBird, SKU: "WH-BIRD"},
	}}
	e.stock = &fakeStock{
		rows: map[uint]*domainStock.Stock{
			skuChicken: {SKUID: skuChicken, Quantity: 12, AvailableQty: 12},
			skuTuna:    {SKUID: skuTuna, Quantity: 30, AvailableQty: 30},
			skuLitter:  {SKUID: skuLitter, Quantity: 40, AvailableQty: 40},
			skuFish:    {SKUID: skuFish, Quantity: 22, AvailableQty: 22},
			skuBird:    {SKUID: skuBird, Quantity: 10, AvailableQty: 10},
		},
		lots: []*domainStock.StockLot{
			{ID: 1, SKUID: skuChicken, LotNumber: "LOT-A", ExpiryDate: "2027-01-31", Quantity: 2},
			{ID: 2, SKUID: skuChicken, LotNumber: "LOT-B", ExpiryDate: "2027-04-30", Quantity: 10},
			{ID: 3, SKUID: skuTuna, LotNumber: "LOT-T", ExpiryDate: "2027-03-01", Quantity: 30},
			{ID: 4, SKUID: skuLitter, LotNumber: "LOT-L", ExpiryDate: "2028-01-01", Quantity: 40},
			{ID: 5, SKUID: skuFish, LotNumber: "LOT-F", ExpiryDate: "2030-01-01", Quantity: 22},
			{ID: 6, SKUID: skuBird, LotNumber: "LOT-W", ExpiryDate: "2030-01-01", Quantity: 10},
		},
	}
	e.uc = New(e.repo, skus, e.stock, fakeTx{}).(*usecase)
	e.clock = time.Date(2026, 11, 26, 10, 42, 5, 0, e.uc.loc)
	e.uc.now = func() time.Time { return e.clock }
	return e
}

func (e *env) count(t *testing.T, id string, who Actor, lines ...CountLine) *BatchResult {
	t.Helper()
	res, err := e.uc.CreateBatch(context.Background(), BatchInput{ClientID: id, Lines: lines, Actor: who})
	require.NoError(t, err)
	return res
}

func boxOf(s *Summary, sku uint) BoxSummary {
	for _, b := range s.Boxes {
		if b.SKUID == sku {
			return b
		}
	}
	return BoxSummary{}
}

// ---- spec test cases ----

// "แตะทรายแมว 2 และหัวเปลี่ยน 2 แบบ แบบละ 1 แล้วบันทึก ยอดออกแยกแบบถูก และยอดรวมหัวไม้ = 2"
func TestPerVariantTotals(t *testing.T) {
	e := newEnv()
	e.count(t, "c1", packer, CountLine{skuLitter, 2}, CountLine{skuFish, 1}, CountLine{skuBird, 1})
	s, err := e.uc.Summary(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, 2, boxOf(s, skuLitter).OutToday)
	require.Equal(t, 1, boxOf(s, skuFish).OutToday)
	require.Equal(t, 1, boxOf(s, skuBird).OutToday)
	heads := 0
	for _, b := range s.Boxes {
		if b.Group == "หัวไม้ตกแมว" {
			heads += b.OutToday
		}
	}
	require.Equal(t, 2, heads)
	require.Equal(t, 38, boxOf(s, skuLitter).OnHand)
}

// "แตะไก่ 3 ทูน่า 1 แล้วบันทึก ไก่ออกวันนี้ +3 ทูน่า +1 คงเหลือลดตามนั้น"
func TestOutTodayAndOnHand(t *testing.T) {
	e := newEnv()
	e.count(t, "c1", packer, CountLine{skuChicken, 3}, CountLine{skuTuna, 1})
	s, _ := e.uc.Summary(context.Background(), "2026-11-26")
	require.Equal(t, 3, boxOf(s, skuChicken).OutToday)
	require.Equal(t, 1, boxOf(s, skuTuna).OutToday)
	require.Equal(t, 9, boxOf(s, skuChicken).OnHand)
	require.Equal(t, 29, boxOf(s, skuTuna).OnHand)
	require.True(t, boxOf(s, skuChicken).Low) // 9 <= 20
	require.Equal(t, "LOT-B", boxOf(s, skuChicken).Lot.LotNumber)
	require.Equal(t, 9, boxOf(s, skuChicken).Lot.Left)
}

// "ไก่ LOT A เหลือ 2 (หมดอายุก่อน) LOT B เหลือ 10 บันทึกไก่ 5 ต้องตัด A 2 + B 3"
func TestFEFOSplitsAcrossLots(t *testing.T) {
	e := newEnv()
	res := e.count(t, "c1", packer, CountLine{skuChicken, 5})
	line := res.Batch.Lines[0]
	require.Equal(t, []domainStockOut.LotTake{
		{LotID: 1, LotNumber: "LOT-A", Expiry: "2027-01-31", Qty: 2},
		{LotID: 2, LotNumber: "LOT-B", Expiry: "2027-04-30", Qty: 3},
	}, line.Lots)
	require.Equal(t, 0, e.stock.lots[0].Quantity)
	require.Equal(t, 7, e.stock.lots[1].Quantity)
	require.Len(t, e.stock.movements, 2)
	for _, m := range e.stock.movements {
		require.NotNil(t, m.StockLotID)
		require.Equal(t, domainStock.MovementOut, m.Type)
		require.Equal(t, "STOCK_OUT", m.Channel)
		require.Equal(t, "SO-000001", m.ReferenceID)
	}
	require.Equal(t, domainStockOut.StatusPosted, res.Batch.Status)
}

// "ส่งรอบเดิมซ้ำ 2 ครั้ง สต็อกลดครั้งเดียว"
func TestResendDeductsOnce(t *testing.T) {
	e := newEnv()
	e.count(t, "dup", packer, CountLine{skuTuna, 4})
	again := e.count(t, "dup", packer, CountLine{skuTuna, 4})
	require.True(t, again.Replayed)
	require.Len(t, e.repo.batches, 1)
	require.Equal(t, 26, e.stock.rows[skuTuna].Quantity)
}

// "บันทึกแล้วกด ยกเลิกรอบนี้ ยอดออกวันนี้และคงเหลือกลับเหมือนเดิม LOT A และ B ได้คืนตามที่ตัด"
func TestUndoRestoresSameLots(t *testing.T) {
	e := newEnv()
	res := e.count(t, "c1", packer, CountLine{skuChicken, 5})
	e.clock = e.clock.Add(5 * time.Second)
	b, err := e.uc.VoidBatch(context.Background(), res.Batch.ID, packer)
	require.NoError(t, err)
	require.Equal(t, domainStockOut.StatusVoided, b.Status)
	require.Equal(t, 2, e.stock.lots[0].Quantity)
	require.Equal(t, 10, e.stock.lots[1].Quantity)
	require.Equal(t, 12, e.stock.rows[skuChicken].Quantity)
	s, _ := e.uc.Summary(context.Background(), "")
	require.Equal(t, 0, boxOf(s, skuChicken).OutToday)
	back := e.stock.movements[2:]
	require.Len(t, back, 2)
	for _, m := range back {
		require.Equal(t, domainStock.MovementIn, m.Type)
		require.Equal(t, "STOCK_OUT_VOID", m.ReferenceType)
	}
}

// Voiding later, or someone else's batch, needs the void role.
func TestVoidPermissions(t *testing.T) {
	e := newEnv()
	res := e.count(t, "c1", packer, CountLine{skuTuna, 1})
	var appErr *appErrors.AppError
	_, err := e.uc.VoidBatch(context.Background(), res.Batch.ID, other)
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "FORBIDDEN", appErr.Code)

	e.clock = e.clock.Add(10 * time.Minute)
	_, err = e.uc.VoidBatch(context.Background(), res.Batch.ID, packer)
	require.Error(t, err)

	_, err = e.uc.VoidBatch(context.Background(), res.Batch.ID, owner)
	require.NoError(t, err)
	_, err = e.uc.VoidBatch(context.Background(), res.Batch.ID, owner)
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "ALREADY_VOIDED", appErr.Code)
}

// "ระบบบอกไก่เหลือ 1 แต่บันทึกออก 3 บันทึกได้ ขึ้น needs_review"
func TestShortStockIsRecordedForReview(t *testing.T) {
	e := newEnv()
	e.stock.rows[skuChicken].Quantity, e.stock.rows[skuChicken].AvailableQty = 1, 1
	e.stock.lots[0].Quantity, e.stock.lots[1].Quantity = 1, 0
	res := e.count(t, "c1", packer, CountLine{skuChicken, 3}, CountLine{skuTuna, 1})
	require.Equal(t, domainStockOut.StatusReview, res.Batch.Status)
	require.Equal(t, 3, res.Batch.Lines[0].Qty)
	require.Equal(t, 1, res.Batch.Lines[0].Deducted)
	require.Equal(t, 2, res.Batch.Lines[0].Short)
	require.Equal(t, 0, e.stock.rows[skuChicken].Quantity) // never below zero
	require.Equal(t, 29, e.stock.rows[skuTuna].Quantity)

	s, _ := e.uc.Summary(context.Background(), "")
	require.Equal(t, 1, s.PendingRev)
	require.Equal(t, 3, boxOf(s, skuChicken).OutToday) // what really left the shelf

	_, err := e.uc.ReviewBatch(context.Background(), res.Batch.ID, owner, "ปรับยอดแล้ว")
	require.NoError(t, err)
	s, _ = e.uc.Summary(context.Background(), "")
	require.Equal(t, 0, s.PendingRev)
}

// Stock without lot records is never taken without naming a lot.
func TestStockWithoutLotsGoesToReview(t *testing.T) {
	e := newEnv()
	e.stock.lots = e.stock.lots[2:] // chicken has stock but no lots
	res := e.count(t, "c1", packer, CountLine{skuChicken, 2})
	require.Equal(t, domainStockOut.StatusReview, res.Batch.Status)
	require.Equal(t, 0, res.Batch.Lines[0].Deducted)
	require.Equal(t, 12, e.stock.rows[skuChicken].Quantity)
	require.Empty(t, e.stock.movements)
}

// "รอบที่บันทึก 23:50 เวลาไทย นับเป็นยอดของวันนั้น ไม่ไปวันถัดไป"
func TestBangkokDayBoundary(t *testing.T) {
	e := newEnv()
	e.clock = time.Date(2026, 11, 26, 23, 50, 0, 0, e.uc.loc) // 16:50 UTC
	e.count(t, "late", packer, CountLine{skuTuna, 2})
	s26, _ := e.uc.Summary(context.Background(), "2026-11-26")
	s27, _ := e.uc.Summary(context.Background(), "2026-11-27")
	require.Equal(t, 2, boxOf(s26, skuTuna).OutToday)
	require.Equal(t, 0, boxOf(s27, skuTuna).OutToday)
	rows, err := e.uc.History(context.Background(), "2026-11-26", "2026-11-27")
	require.NoError(t, err)
	require.Equal(t, []HistoryRow{{Date: "2026-11-26", SKUID: skuTuna, Qty: 2}}, rows)
}

// "เน็ตหลุด บันทึก 5 รอบ พอต่อเน็ตกลับมา ส่งครบ 5 รอบและไม่ซ้ำ" — the device
// time of an offline batch decides its day; a retry of the same id is a no-op.
func TestOfflineBatchesKeepTheirCountedTime(t *testing.T) {
	e := newEnv()
	tapped := e.clock.Add(-26 * time.Hour) // counted yesterday, synced today
	for i, id := range []string{"o1", "o2", "o3", "o4", "o5"} {
		at := tapped.Add(time.Duration(i) * time.Minute)
		_, err := e.uc.CreateBatch(context.Background(), BatchInput{ClientID: id, CountedAt: &at, Lines: []CountLine{{skuTuna, 1}}, Actor: packer})
		require.NoError(t, err)
	}
	at := tapped
	again, err := e.uc.CreateBatch(context.Background(), BatchInput{ClientID: "o1", CountedAt: &at, Lines: []CountLine{{skuTuna, 1}}, Actor: packer})
	require.NoError(t, err)
	require.True(t, again.Replayed)
	require.Equal(t, 25, e.stock.rows[skuTuna].Quantity)
	yesterday, _ := e.uc.Summary(context.Background(), "2026-11-25")
	require.Equal(t, 5, boxOf(yesterday, skuTuna).OutToday)

	future := e.clock.Add(time.Hour)
	res, err := e.uc.CreateBatch(context.Background(), BatchInput{ClientID: "f", CountedAt: &future, Lines: []CountLine{{skuTuna, 1}}, Actor: packer})
	require.NoError(t, err)
	require.Equal(t, e.clock, res.Batch.CountedAt) // a clock far ahead is not trusted
}

// "แตะ − จนเหลือ 0 ทุกกล่อง ปุ่มบันทึกกดไม่ได้" — the API refuses an empty batch too.
func TestEmptyOrUnknownBatchIsRefused(t *testing.T) {
	e := newEnv()
	_, err := e.uc.CreateBatch(context.Background(), BatchInput{ClientID: "x", Actor: packer})
	require.Error(t, err)
	_, err = e.uc.CreateBatch(context.Background(), BatchInput{ClientID: "y", Lines: []CountLine{{999, 1}}, Actor: packer})
	require.Error(t, err)
	_, err = e.uc.CreateBatch(context.Background(), BatchInput{ClientID: "z", Lines: []CountLine{{skuTuna, 1}}, Actor: Actor{ID: 9, Role: "live"}})
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "FORBIDDEN", appErr.Code)
	require.Empty(t, e.repo.batches)
}

func TestSaveConfigValidates(t *testing.T) {
	e := newEnv()
	cfg := DefaultConfig()
	cfg.Boxes = []domainStockOut.Box{{SKUID: skuTuna, Name: "ทูน่า", Kind: "flavor", Color: "blue"}}
	_, err := e.uc.SaveConfig(context.Background(), cfg, "owner")
	require.Error(t, err)

	cfg.Boxes = []domainStockOut.Box{{SKUID: skuTuna, Name: "ทูน่า", Kind: "flavor", Color: "#0047B8"}, {SKUID: skuTuna, Name: "ซ้ำ", Kind: "item"}}
	_, err = e.uc.SaveConfig(context.Background(), cfg, "owner")
	require.Error(t, err)

	cfg.Boxes = []domainStockOut.Box{{SKUID: skuTuna, Name: "ทูน่า", Kind: "flavor", Color: "#0047B8"}}
	cfg.CountRoles = []string{"warehouse"}
	cfg.VoidRoles = []string{}
	v, err := e.uc.SaveConfig(context.Background(), cfg, "owner")
	require.NoError(t, err)
	require.Equal(t, []string{"owner", "warehouse"}, v.Config.CountRoles)
	require.Equal(t, []string{"owner"}, v.Config.VoidRoles)
	require.Equal(t, "ชิ้น", v.Config.Boxes[0].Unit)
	require.Equal(t, "TRT-TUN", v.SKUCodes[skuTuna])

	cfg.CountRoles = []string{"admin"}
	_, err = e.uc.SaveConfig(context.Background(), cfg, "owner")
	require.Error(t, err)
}
