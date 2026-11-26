package booth

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	domainBooth "chawy-erp-api/internal/domain/booth"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	pkgDatabase "chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"
)

// SKUFinder is the part of the SKU repository the booth reads.
type SKUFinder interface {
	FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error)
}

// StockStore is the part of the stock repository the booth uses: the same
// stock rows, lots and movements the inventory page reads.
type StockStore interface {
	GetBySKUID(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error)
	GetBySKUIDForUpdate(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error)
	UpdateQuantity(ctx context.Context, skuID, warehouseID uint, delta int) (*domainStock.Stock, error)
	CreateMovement(ctx context.Context, m *domainStock.StockMovement) error
	GetAvailableLotsForUpdate(ctx context.Context, skuID, warehouseID uint) ([]domainStock.StockLot, error)
	DeductLotQuantity(ctx context.Context, lotID uint, qty int) (*domainStock.StockLot, error)
}

// Button is one product button on the sell screen with its stock right now.
type Button struct {
	SKUID     uint    `json:"skuId"`
	SKUCode   string  `json:"skuCode"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Color     string  `json:"color,omitempty"`
	Price     float64 `json:"price,omitempty"`
	Available int     `json:"available"`
	Missing   bool    `json:"missing,omitempty"`
}

// Catalog is the config plus the buttons with stock.
type Catalog struct {
	Config    domainBooth.Config `json:"config"`
	Buttons   []Button           `json:"buttons"`
	UpdatedAt *time.Time         `json:"updatedAt"`
	UpdatedBy string             `json:"updatedBy"`
}

// CartLine is a requested SKU quantity.
type CartLine struct {
	SKUID uint
	Qty   int
}

// SaleInput is one bill from the booth screen.
type SaleInput struct {
	ClientRef     string
	Lines         []CartLine
	PaymentMethod string
	CashReceived  *float64
	ExpectedTotal *float64
	SellerID      uint
	SellerName    string
}

// SaleResult is the stored bill; Replayed is true when the client ref was
// already used and nothing was deducted again.
type SaleResult struct {
	Sale     *domainBooth.Sale `json:"sale"`
	Replayed bool              `json:"replayed"`
}

// Shortage is one SKU without enough available stock.
type Shortage struct {
	SKUID     uint   `json:"skuId"`
	SKUCode   string `json:"skuCode"`
	Name      string `json:"name"`
	Requested int    `json:"requested"`
	Available int    `json:"available"`
}

// InsufficientStockError lists every short SKU; nothing was deducted.
type InsufficientStockError struct{ Shortages []Shortage }

func (e *InsufficientStockError) Error() string {
	parts := make([]string, 0, len(e.Shortages))
	for _, s := range e.Shortages {
		parts = append(parts, fmt.Sprintf("%s ต้องการ %d เหลือ %d", s.Name, s.Requested, s.Available))
	}
	return "สต็อกไม่พอ: " + strings.Join(parts, ", ")
}

func (e *InsufficientStockError) Unwrap() error { return appErrors.ErrInsufficientStock }

// PriceChangedError means the screen showed a different total than the API
// computes now (the config changed); nothing was deducted.
type PriceChangedError struct{ Bill *Bill }

func (e *PriceChangedError) Error() string {
	return fmt.Sprintf("ราคาเปลี่ยนแล้ว ยอดที่ถูกต้องคือ %.2f", e.Bill.Total)
}

// Usecase is the booth point of sale.
type Usecase interface {
	GetCatalog(ctx context.Context) (*Catalog, error)
	UpdateConfig(ctx context.Context, cfg domainBooth.Config, actor string) (*Catalog, error)
	Quote(ctx context.Context, lines []CartLine) (*Bill, error)
	CreateSale(ctx context.Context, in SaleInput) (*SaleResult, error)
	VoidSale(ctx context.Context, id uint, actor, reason string) (*domainBooth.Sale, error)
	ListSales(ctx context.Context, date string) ([]domainBooth.Sale, error)
}

type usecase struct {
	repo  domainBooth.Repository
	skus  SKUFinder
	stock StockStore
	tx    pkgDatabase.TxManager
	now   func() time.Time
	loc   *time.Location
}

// New builds the booth usecase.
func New(repo domainBooth.Repository, skus SKUFinder, stock StockStore, tx pkgDatabase.TxManager) Usecase {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	return &usecase{repo: repo, skus: skus, stock: stock, tx: tx, now: time.Now, loc: loc}
}

// DefaultConfig is used until the owner saves one: the set prices the booth
// already used, no buttons yet (each button needs an SKU from the ERP).
func DefaultConfig() domainBooth.Config {
	return domainBooth.Config{
		WarehouseID: domainBooth.DefaultWarehouse,
		Flavors:     []domainBooth.Flavor{},
		Sets: []domainBooth.SetPrice{
			{Size: 1, Price: 129}, {Size: 3, Price: 289}, {Size: 5, Price: 459}, {Size: 10, Price: 890},
		},
		Items:    []domainBooth.Item{},
		Discount: domainBooth.Discount{Enabled: true, Min: 1000, Type: domainBooth.DiscountBaht, Value: 50},
	}
}

func (u *usecase) loadConfig(ctx context.Context) (domainBooth.Config, *domainBooth.ConfigRecord, error) {
	rec, err := u.repo.GetConfig(ctx)
	if err != nil {
		return domainBooth.Config{}, nil, err
	}
	if rec == nil {
		return DefaultConfig(), nil, nil
	}
	cfg := rec.Data
	if cfg.WarehouseID == 0 {
		cfg.WarehouseID = domainBooth.DefaultWarehouse
	}
	return cfg, rec, nil
}

func (u *usecase) GetCatalog(ctx context.Context) (*Catalog, error) {
	cfg, rec, err := u.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return u.catalog(ctx, cfg, rec)
}

func (u *usecase) catalog(ctx context.Context, cfg domainBooth.Config, rec *domainBooth.ConfigRecord) (*Catalog, error) {
	out := &Catalog{Config: cfg, Buttons: []Button{}}
	if rec != nil {
		t := rec.UpdatedAt
		out.UpdatedAt, out.UpdatedBy = &t, rec.UpdatedBy
	}
	add := func(id uint, name, kind, color string, price float64) error {
		b := Button{SKUID: id, Name: name, Kind: kind, Color: color, Price: price}
		s, err := u.skus.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if s == nil {
			b.Missing = true
		} else {
			b.SKUCode = s.SKU
			stk, err := u.stock.GetBySKUID(ctx, id, cfg.WarehouseID)
			if err != nil {
				return err
			}
			if stk != nil {
				b.Available = stk.AvailableQty
			}
		}
		out.Buttons = append(out.Buttons, b)
		return nil
	}
	for _, f := range cfg.Flavors {
		if err := add(f.SKUID, f.Name, domainBooth.KindFlavor, f.Color, 0); err != nil {
			return nil, err
		}
	}
	for _, it := range cfg.Items {
		if err := add(it.SKUID, it.Name, domainBooth.KindItem, "", it.Price); err != nil {
			return nil, err
		}
	}
	return out, nil
}

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (u *usecase) validateConfig(ctx context.Context, cfg domainBooth.Config) (domainBooth.Config, error) {
	bad := func(msg string) error { return appErrors.NewAppError("VALIDATION_ERROR", msg, 400) }
	if cfg.WarehouseID == 0 {
		cfg.WarehouseID = domainBooth.DefaultWarehouse
	}
	if len(cfg.Flavors)+len(cfg.Items) > domainBooth.MaxConfigRows || len(cfg.Sets) > domainBooth.MaxConfigRows {
		return cfg, bad("too many buttons or set prices")
	}
	seen := map[uint]bool{}
	checkSKU := func(id uint, name string) error {
		if id == 0 {
			return bad("every button needs an SKU")
		}
		if seen[id] {
			return bad(fmt.Sprintf("SKU %d is used on two buttons", id))
		}
		seen[id] = true
		if strings.TrimSpace(name) == "" || len([]rune(name)) > 60 {
			return bad("button names need 1–60 characters")
		}
		s, err := u.skus.FindByID(ctx, id)
		if err != nil {
			return err
		}
		if s == nil {
			return appErrors.NewAppError("SKU_NOT_FOUND", fmt.Sprintf("SKU %d not found", id), 400)
		}
		return nil
	}
	for i, f := range cfg.Flavors {
		if err := checkSKU(f.SKUID, f.Name); err != nil {
			return cfg, err
		}
		if !colorRe.MatchString(f.Color) {
			return cfg, bad("flavor colors must look like #D71920")
		}
		cfg.Flavors[i].Name = strings.TrimSpace(f.Name)
	}
	for i, it := range cfg.Items {
		if err := checkSKU(it.SKUID, it.Name); err != nil {
			return cfg, err
		}
		if it.Price < 0 {
			return cfg, bad("prices cannot be negative")
		}
		cfg.Items[i].Name = strings.TrimSpace(it.Name)
	}
	sizes := map[int]bool{}
	for _, s := range cfg.Sets {
		if s.Size <= 0 || s.Size > 100 || s.Price < 0 || sizes[s.Size] {
			return cfg, bad("set sizes must be 1–100, once each, with a price of 0 or more")
		}
		sizes[s.Size] = true
	}
	if !sizes[1] {
		return cfg, appErrors.NewAppError("BOOTH_CONFIG_INVALID", "set prices need a set of 1 bag", 400)
	}
	sort.Slice(cfg.Sets, func(i, j int) bool { return cfg.Sets[i].Size < cfg.Sets[j].Size })
	d := cfg.Discount
	if d.Type != domainBooth.DiscountBaht && d.Type != domainBooth.DiscountPct {
		return cfg, bad("discount type must be baht or percent")
	}
	if d.Min < 0 || d.Value < 0 || (d.Type == domainBooth.DiscountPct && d.Value > 100) {
		return cfg, bad("discount values are out of range")
	}
	return cfg, nil
}

func (u *usecase) UpdateConfig(ctx context.Context, cfg domainBooth.Config, actor string) (*Catalog, error) {
	cfg, err := u.validateConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	rec := &domainBooth.ConfigRecord{ID: domainBooth.ConfigRowID, Data: cfg, UpdatedBy: actor, UpdatedAt: u.now()}
	if err := u.repo.SaveConfig(ctx, rec); err != nil {
		return nil, err
	}
	return u.catalog(ctx, cfg, rec)
}

func mergeLines(lines []CartLine) (map[uint]int, error) {
	if len(lines) == 0 || len(lines) > domainBooth.MaxLines {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("a bill needs 1–%d lines", domainBooth.MaxLines), 400)
	}
	qty := map[uint]int{}
	for _, l := range lines {
		if l.SKUID == 0 || l.Qty <= 0 {
			return nil, appErrors.NewAppError("VALIDATION_ERROR", "every line needs skuId and a positive qty", 400)
		}
		qty[l.SKUID] += l.Qty
		if qty[l.SKUID] > domainBooth.MaxQtyPerLine {
			return nil, appErrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("at most %d per SKU on one bill", domainBooth.MaxQtyPerLine), 400)
		}
	}
	return qty, nil
}

func (u *usecase) Quote(ctx context.Context, lines []CartLine) (*Bill, error) {
	qty, err := mergeLines(lines)
	if err != nil {
		return nil, err
	}
	cfg, _, err := u.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return PriceCart(cfg, qty)
}

func validateSale(in SaleInput) (SaleInput, map[uint]int, error) {
	in.ClientRef = strings.TrimSpace(in.ClientRef)
	if in.ClientRef == "" || len(in.ClientRef) > domainBooth.MaxClientRef {
		return in, nil, appErrors.NewAppError("VALIDATION_ERROR", "clientRef is required (64 characters max)", 400)
	}
	switch in.PaymentMethod {
	case domainBooth.PayCash, domainBooth.PayTransfer, domainBooth.PayCard:
	default:
		return in, nil, appErrors.NewAppError("VALIDATION_ERROR", "paymentMethod must be cash, transfer or card", 400)
	}
	qty, err := mergeLines(in.Lines)
	return in, qty, err
}

// planned is one locked line ready to deduct.
type planned struct {
	line *domainBooth.SaleLine
	sku  *domainSKU.SKU
	stk  *domainStock.Stock
}

// CreateSale prices the bill, checks every SKU under a stock-row lock and
// takes the quantities out of the inventory (stock row, FEFO lots, one OUT
// movement per lot) in one transaction. A client ref that was already used
// returns the first bill and deducts nothing. If any SKU is short, nothing is
// deducted and every short SKU is reported.
func (u *usecase) CreateSale(ctx context.Context, in SaleInput) (*SaleResult, error) {
	in, qty, err := validateSale(in)
	if err != nil {
		return nil, err
	}
	if existing, err := u.repo.FindByClientRef(ctx, in.ClientRef); err != nil {
		return nil, err
	} else if existing != nil {
		return &SaleResult{Sale: existing, Replayed: true}, nil
	}

	var result *SaleResult
	err = u.tx.Transaction(ctx, func(txCtx context.Context) error {
		if existing, err := u.repo.FindByClientRef(txCtx, in.ClientRef); err != nil {
			return err
		} else if existing != nil {
			result = &SaleResult{Sale: existing, Replayed: true}
			return nil
		}
		cfg, _, err := u.loadConfig(txCtx)
		if err != nil {
			return err
		}
		bill, err := PriceCart(cfg, qty)
		if err != nil {
			return err
		}
		if in.ExpectedTotal != nil && math.Abs(*in.ExpectedTotal-bill.Total) > 0.005 {
			return &PriceChangedError{Bill: bill}
		}
		sale := &domainBooth.Sale{
			ClientRef: in.ClientRef, WarehouseID: cfg.WarehouseID,
			SellerID: in.SellerID, SellerName: in.SellerName,
			Lines: bill.Lines, Bags: bill.Bags, TreatLabel: bill.TreatLabel, TreatParts: bill.TreatParts,
			TreatTotal: bill.TreatTotal, ItemsTotal: bill.ItemsTotal, Subtotal: bill.Subtotal,
			Discount: bill.Discount, Total: bill.Total, PaymentMethod: in.PaymentMethod,
			Status: domainBooth.StatusDone, CreatedAt: u.now(),
		}
		if in.PaymentMethod == domainBooth.PayCash {
			received := bill.Total
			if in.CashReceived != nil {
				received = *in.CashReceived
			}
			if received < bill.Total-0.005 {
				return appErrors.NewAppError("CASH_SHORT", fmt.Sprintf("รับเงินไม่พอ ยอด %.2f รับมา %.2f", bill.Total, received), 400)
			}
			sale.CashReceived, sale.Change = round2(received), round2(received-bill.Total)
		}

		// Lock stock rows in SKU order so two bills never wait on each other.
		order := make([]int, len(sale.Lines))
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(a, b int) bool { return sale.Lines[order[a]].SKUID < sale.Lines[order[b]].SKUID })
		plans := make([]planned, 0, len(order))
		var shortages []Shortage
		for _, i := range order {
			line := &sale.Lines[i]
			s, err := u.skus.FindByID(txCtx, line.SKUID)
			if err != nil {
				return err
			}
			if s == nil {
				return appErrors.NewAppError("SKU_NOT_FOUND", fmt.Sprintf("SKU %d not found", line.SKUID), 404)
			}
			line.SKUCode = s.SKU
			stk, err := u.stock.GetBySKUIDForUpdate(txCtx, s.ID, cfg.WarehouseID)
			if err != nil {
				return err
			}
			available, total := 0, 0
			if stk != nil {
				available, total = stk.AvailableQty, stk.Quantity
			}
			if available < line.Qty || total < line.Qty {
				shortages = append(shortages, Shortage{SKUID: s.ID, SKUCode: s.SKU, Name: line.Name, Requested: line.Qty, Available: available})
				continue
			}
			lots, err := u.stock.GetAvailableLotsForUpdate(txCtx, s.ID, cfg.WarehouseID)
			if err != nil {
				return err
			}
			line.Lots = allocateLots(lots, line.Qty)
			plans = append(plans, planned{line: line, sku: s, stk: stk})
		}
		if len(shortages) > 0 {
			return &InsufficientStockError{Shortages: shortages}
		}

		if err := u.repo.Create(txCtx, sale); err != nil {
			return err
		}
		sale.SaleNo = fmt.Sprintf("BTH-%06d", sale.ID)
		if err := u.repo.SetSaleNo(txCtx, sale.ID, sale.SaleNo); err != nil {
			return err
		}
		for _, p := range plans {
			if err := u.moveOut(txCtx, sale, p, cfg.WarehouseID); err != nil {
				return err
			}
		}
		result = &SaleResult{Sale: sale}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// allocateLots takes qty from the earliest-expiring lots first (FEFO). What
// the lots cannot cover is legacy stock without lot records.
func allocateLots(lots []domainStock.StockLot, qty int) []domainBooth.LotAlloc {
	var out []domainBooth.LotAlloc
	for _, lot := range lots {
		if qty <= 0 {
			break
		}
		free := lot.Quantity - lot.ReservedQty
		if free <= 0 {
			continue
		}
		take := free
		if take > qty {
			take = qty
		}
		out = append(out, domainBooth.LotAlloc{LotID: lot.ID, LotNumber: lot.LotNumber, Qty: take})
		qty -= take
	}
	return out
}

func (u *usecase) moveOut(ctx context.Context, sale *domainBooth.Sale, p planned, wh uint) error {
	if _, err := u.stock.UpdateQuantity(ctx, p.sku.ID, wh, -p.line.Qty); err != nil {
		return err
	}
	current := p.stk.Quantity
	movement := func(lot *domainBooth.LotAlloc, qty int, note string) error {
		m := &domainStock.StockMovement{
			SKUID: p.sku.ID, SKUCode: p.sku.SKU, WarehouseID: wh, Channel: domainBooth.Channel,
			Type: domainStock.MovementOut, Quantity: qty, BeforeQty: current, AfterQty: current - qty,
			ReferenceType: domainBooth.RefTypeSale, ReferenceID: sale.SaleNo, Note: note, CreatedAt: u.now(),
		}
		if lot != nil {
			id := lot.LotID
			m.StockLotID = &id
		}
		if err := u.stock.CreateMovement(ctx, m); err != nil {
			return err
		}
		current -= qty
		return nil
	}
	left := p.line.Qty
	for i := range p.line.Lots {
		lot := &p.line.Lots[i]
		if _, err := u.stock.DeductLotQuantity(ctx, lot.LotID, lot.Qty); err != nil {
			return err
		}
		if err := movement(lot, lot.Qty, fmt.Sprintf("ขายหน้าบูธ %s (Lot: %s)", sale.SaleNo, lot.LotNumber)); err != nil {
			return err
		}
		left -= lot.Qty
	}
	if left > 0 {
		return movement(nil, left, "ขายหน้าบูธ "+sale.SaleNo)
	}
	return nil
}

// VoidSale keeps the bill, marks it voided and puts every quantity back into
// the same stock row and lots, with an IN movement that cites the bill.
func (u *usecase) VoidSale(ctx context.Context, id uint, actor, reason string) (*domainBooth.Sale, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) > 255 {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", "reason is too long (255 max)", 400)
	}
	var out *domainBooth.Sale
	err := u.tx.Transaction(ctx, func(txCtx context.Context) error {
		sale, err := u.repo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if sale == nil {
			return appErrors.NewAppError("NOT_FOUND", "bill not found", 404)
		}
		if sale.Status == domainBooth.StatusVoided {
			return appErrors.NewAppError("ALREADY_VOIDED", "this bill is already voided", 409)
		}
		lines := append([]domainBooth.SaleLine(nil), sale.Lines...)
		sort.Slice(lines, func(a, b int) bool { return lines[a].SKUID < lines[b].SKUID })
		for _, l := range lines {
			stk, err := u.stock.GetBySKUIDForUpdate(txCtx, l.SKUID, sale.WarehouseID)
			if err != nil {
				return err
			}
			current := 0
			if stk != nil {
				current = stk.Quantity
			}
			if _, err := u.stock.UpdateQuantity(txCtx, l.SKUID, sale.WarehouseID, l.Qty); err != nil {
				return err
			}
			for _, lot := range l.Lots {
				if _, err := u.stock.DeductLotQuantity(txCtx, lot.LotID, -lot.Qty); err != nil {
					return err
				}
			}
			if err := u.stock.CreateMovement(txCtx, &domainStock.StockMovement{
				SKUID: l.SKUID, SKUCode: l.SKUCode, WarehouseID: sale.WarehouseID, Channel: domainBooth.Channel,
				Type: domainStock.MovementIn, Quantity: l.Qty, BeforeQty: current, AfterQty: current + l.Qty,
				ReferenceType: domainBooth.RefTypeVoid, ReferenceID: sale.SaleNo,
				Note: truncate("ยกเลิกบิลบูธ "+sale.SaleNo+" คืนสต็อก "+reason, 255), CreatedAt: u.now(),
			}); err != nil {
				return err
			}
		}
		at := u.now()
		if err := u.repo.MarkVoided(txCtx, sale.ID, actor, reason, at); err != nil {
			return err
		}
		sale.Status, sale.VoidedAt, sale.VoidedBy, sale.VoidReason = domainBooth.StatusVoided, &at, actor, reason
		out = sale
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListSales returns the bills of one day (YYYY-MM-DD, Bangkok time), oldest
// first; an empty date means today.
func (u *usecase) ListSales(ctx context.Context, date string) ([]domainBooth.Sale, error) {
	var day time.Time
	if strings.TrimSpace(date) == "" {
		n := u.now().In(u.loc)
		day = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, u.loc)
	} else {
		d, err := time.ParseInLocation("2006-01-02", date, u.loc)
		if err != nil {
			return nil, appErrors.NewAppError("VALIDATION_ERROR", "date must be YYYY-MM-DD", 400)
		}
		day = d
	}
	return u.repo.List(ctx, domainBooth.SaleFilter{From: day, To: day.AddDate(0, 0, 1), Limit: 2000})
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	for len(string(r)) > n {
		r = r[:len(r)-1]
	}
	return string(r)
}
