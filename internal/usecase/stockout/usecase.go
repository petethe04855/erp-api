package stockout

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainStockOut "chawy-erp-api/internal/domain/stockout"
	pkgDatabase "chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"
)

// SKUFinder is the part of the SKU repository the count reads.
type SKUFinder interface {
	FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error)
}

// StockStore is the part of the stock repository the count uses: the same
// stock rows, lots and movements the inventory page reads.
type StockStore interface {
	GetBySKUID(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error)
	GetBySKUIDForUpdate(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error)
	UpdateQuantity(ctx context.Context, skuID, warehouseID uint, delta int) (*domainStock.Stock, error)
	CreateMovement(ctx context.Context, m *domainStock.StockMovement) error
	GetAvailableLotsForUpdate(ctx context.Context, skuID, warehouseID uint) ([]domainStock.StockLot, error)
	DeductLotQuantity(ctx context.Context, lotID uint, qty int) (*domainStock.StockLot, error)
	FindLotsBySKU(ctx context.Context, skuID, warehouseID uint) ([]domainStock.StockLot, error)
}

// Actor is the signed-in ERP user.
type Actor struct {
	ID    uint
	Name  string
	Role  string
	Email string
}

// CountLine is one tapped SKU quantity.
type CountLine struct {
	SKUID uint
	Qty   int
}

// BatchInput is one saved count from a device.
type BatchInput struct {
	ClientID  string
	CountedAt *time.Time
	Lines     []CountLine
	Actor     Actor
}

// BatchResult is the stored batch; Replayed means the client id was already
// used and nothing was taken out again.
type BatchResult struct {
	Batch    *domainStockOut.Batch
	Replayed bool
}

// ActiveLot is the lot the next count will take from.
type ActiveLot struct {
	LotID     uint   `json:"lotId"`
	LotNumber string `json:"lotNumber"`
	Expiry    string `json:"expiry"`
	Left      int    `json:"left"`
}

// BoxSummary is one tile's numbers for a day.
type BoxSummary struct {
	domainStockOut.Box
	SKUCode  string     `json:"skuCode"`
	Missing  bool       `json:"missing,omitempty"`
	OutToday int        `json:"outToday"`
	OnHand   int        `json:"onHand"`
	Low      bool       `json:"low"`
	Lot      *ActiveLot `json:"lot"`
}

// Summary is what the count screen polls.
type Summary struct {
	Date        string       `json:"date"`
	WarehouseID uint         `json:"warehouseId"`
	Boxes       []BoxSummary `json:"boxes"`
	PendingRev  int          `json:"pendingReview"`
	AsOf        time.Time    `json:"asOf"`
}

// HistoryRow is how much of one SKU went out on one day.
type HistoryRow struct {
	Date  string `json:"date"`
	SKUID uint   `json:"skuId"`
	Qty   int    `json:"qty"`
}

// ConfigView is the config with when and by whom it was saved.
type ConfigView struct {
	Config    domainStockOut.Config `json:"config"`
	SKUCodes  map[uint]string       `json:"skuCodes"`
	UpdatedAt *time.Time            `json:"updatedAt"`
	UpdatedBy string                `json:"updatedBy"`
}

// Usecase is the daily stock-out count.
type Usecase interface {
	GetConfig(ctx context.Context) (*ConfigView, error)
	SaveConfig(ctx context.Context, cfg domainStockOut.Config, actor string) (*ConfigView, error)
	Summary(ctx context.Context, date string) (*Summary, error)
	CreateBatch(ctx context.Context, in BatchInput) (*BatchResult, error)
	VoidBatch(ctx context.Context, id uint, actor Actor) (*domainStockOut.Batch, error)
	ReviewBatch(ctx context.Context, id uint, actor Actor, note string) (*domainStockOut.Batch, error)
	ListBatches(ctx context.Context, date string) ([]domainStockOut.Batch, error)
	History(ctx context.Context, from, to string) ([]HistoryRow, error)
}

// ReviewNotifier tells the owner that a count was short. It must not block:
// the batch is already saved when it is called.
type ReviewNotifier interface {
	NotifyReview(batch domainStockOut.Batch)
}

type usecase struct {
	repo     domainStockOut.Repository
	skus     SKUFinder
	stock    StockStore
	tx       pkgDatabase.TxManager
	broker   *Broker
	notifier ReviewNotifier
	now      func() time.Time
	loc      *time.Location
}

// Option adds an optional collaborator.
type Option func(*usecase)

// WithBroker publishes every change to open count screens.
func WithBroker(b *Broker) Option { return func(u *usecase) { u.broker = b } }

// WithNotifier tells the owner about short counts.
func WithNotifier(n ReviewNotifier) Option { return func(u *usecase) { u.notifier = n } }

// New builds the stock-out usecase.
func New(repo domainStockOut.Repository, skus SKUFinder, stock StockStore, tx pkgDatabase.TxManager, opts ...Option) Usecase {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.FixedZone("ICT", 7*3600)
	}
	u := &usecase{repo: repo, skus: skus, stock: stock, tx: tx, now: time.Now, loc: loc}
	for _, o := range opts {
		o(u)
	}
	return u
}

func (u *usecase) publish(kind string, b *domainStockOut.Batch) {
	if u.broker == nil {
		return
	}
	e := Event{Type: kind, At: u.now()}
	if b != nil {
		e.BatchID = b.ID
		e.Date = b.CountedAt.In(u.loc).Format("2006-01-02")
	}
	u.broker.Publish(e)
}

var validRoles = map[string]bool{"owner": true, "warehouse": true, "sales": true, "accountant": true, "live": true}

// DefaultConfig is used until the owner saves one: no boxes yet (each box
// needs an SKU), packers in warehouse or sales may count, only the owner
// may void older batches.
func DefaultConfig() domainStockOut.Config {
	return domainStockOut.Config{
		WarehouseID: domainStockOut.DefaultWarehouse,
		Boxes:       []domainStockOut.Box{},
		CountRoles:  []string{"owner", "warehouse", "sales"},
		VoidRoles:   []string{"owner"},
	}
}

func hasRole(roles []string, role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "owner" {
		return true
	}
	for _, r := range roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}

func (u *usecase) loadConfig(ctx context.Context) (domainStockOut.Config, *domainStockOut.ConfigRecord, error) {
	rec, err := u.repo.GetConfig(ctx)
	if err != nil {
		return domainStockOut.Config{}, nil, err
	}
	if rec == nil {
		return DefaultConfig(), nil, nil
	}
	cfg := rec.Data
	if cfg.WarehouseID == 0 {
		cfg.WarehouseID = domainStockOut.DefaultWarehouse
	}
	if len(cfg.CountRoles) == 0 {
		cfg.CountRoles = DefaultConfig().CountRoles
	}
	if len(cfg.VoidRoles) == 0 {
		cfg.VoidRoles = DefaultConfig().VoidRoles
	}
	return cfg, rec, nil
}

func (u *usecase) view(ctx context.Context, cfg domainStockOut.Config, rec *domainStockOut.ConfigRecord) (*ConfigView, error) {
	v := &ConfigView{Config: cfg, SKUCodes: map[uint]string{}}
	if rec != nil {
		t := rec.UpdatedAt
		v.UpdatedAt, v.UpdatedBy = &t, rec.UpdatedBy
	}
	for _, b := range cfg.Boxes {
		s, err := u.skus.FindByID(ctx, b.SKUID)
		if err != nil {
			return nil, err
		}
		if s != nil {
			v.SKUCodes[b.SKUID] = s.SKU
		}
	}
	return v, nil
}

func (u *usecase) GetConfig(ctx context.Context) (*ConfigView, error) {
	cfg, rec, err := u.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	return u.view(ctx, cfg, rec)
}

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (u *usecase) SaveConfig(ctx context.Context, cfg domainStockOut.Config, actor string) (*ConfigView, error) {
	bad := func(msg string) error { return appErrors.NewAppError("VALIDATION_ERROR", msg, 400) }
	if cfg.WarehouseID == 0 {
		cfg.WarehouseID = domainStockOut.DefaultWarehouse
	}
	if len(cfg.Boxes) > domainStockOut.MaxBoxes {
		return nil, bad(fmt.Sprintf("at most %d boxes", domainStockOut.MaxBoxes))
	}
	seen := map[uint]bool{}
	for i, b := range cfg.Boxes {
		b.Name, b.Group, b.Unit, b.Image = strings.TrimSpace(b.Name), strings.TrimSpace(b.Group), strings.TrimSpace(b.Unit), strings.TrimSpace(b.Image)
		if b.SKUID == 0 {
			return nil, bad("every box needs an SKU")
		}
		if seen[b.SKUID] {
			return nil, bad(fmt.Sprintf("SKU %d is used on two boxes", b.SKUID))
		}
		seen[b.SKUID] = true
		if b.Name == "" || len([]rune(b.Name)) > 60 || len([]rune(b.Group)) > 40 || len([]rune(b.Unit)) > 20 || len(b.Image) > 500 {
			return nil, bad("box name (1–60), group (40), unit (20) or image (500) is out of range")
		}
		if b.Kind != domainStockOut.KindFlavor && b.Kind != domainStockOut.KindItem {
			return nil, bad("box kind must be flavor or item")
		}
		if b.Kind == domainStockOut.KindFlavor && !colorRe.MatchString(b.Color) {
			return nil, bad("flavor colors must look like #D71920")
		}
		if b.LowStock < 0 {
			return nil, bad("low-stock level cannot be negative")
		}
		if b.Unit == "" {
			b.Unit = "ชิ้น"
		}
		s, err := u.skus.FindByID(ctx, b.SKUID)
		if err != nil {
			return nil, err
		}
		if s == nil {
			return nil, appErrors.NewAppError("SKU_NOT_FOUND", fmt.Sprintf("SKU %d not found", b.SKUID), 400)
		}
		cfg.Boxes[i] = b
	}
	clean := func(roles []string, what string) ([]string, error) {
		// The owner can always count and void, so it is always listed.
		out := []string{"owner"}
		for _, r := range roles {
			r = strings.ToLower(strings.TrimSpace(r))
			if !validRoles[r] {
				return nil, bad(fmt.Sprintf("unknown role %q in %s", r, what))
			}
			out = append(out, r)
		}
		return dedupe(out), nil
	}
	var err error
	if cfg.CountRoles, err = clean(cfg.CountRoles, "countRoles"); err != nil {
		return nil, err
	}
	if cfg.VoidRoles, err = clean(cfg.VoidRoles, "voidRoles"); err != nil {
		return nil, err
	}
	rec := &domainStockOut.ConfigRecord{ID: domainStockOut.ConfigRowID, Data: cfg, UpdatedBy: actor, UpdatedAt: u.now()}
	if err := u.repo.SaveConfig(ctx, rec); err != nil {
		return nil, err
	}
	u.publish("config", nil)
	return u.view(ctx, cfg, rec)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, r := range in {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// dayRange is [00:00, 24:00) Bangkok time for YYYY-MM-DD (today when empty).
func (u *usecase) dayRange(date string) (time.Time, string, error) {
	if strings.TrimSpace(date) == "" {
		n := u.now().In(u.loc)
		d := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, u.loc)
		return d, d.Format("2006-01-02"), nil
	}
	d, err := time.ParseInLocation("2006-01-02", date, u.loc)
	if err != nil {
		return time.Time{}, "", appErrors.NewAppError("VALIDATION_ERROR", "date must be YYYY-MM-DD", 400)
	}
	return d, date, nil
}

func (u *usecase) ListBatches(ctx context.Context, date string) ([]domainStockOut.Batch, error) {
	day, _, err := u.dayRange(date)
	if err != nil {
		return nil, err
	}
	return u.repo.List(ctx, domainStockOut.Filter{From: day, To: day.AddDate(0, 0, 1), Limit: 5000})
}

// counted is what a batch took off the shelf: voided batches count nothing.
func counted(b domainStockOut.Batch) bool { return b.Status != domainStockOut.StatusVoided }

func (u *usecase) Summary(ctx context.Context, date string) (*Summary, error) {
	day, label, err := u.dayRange(date)
	if err != nil {
		return nil, err
	}
	cfg, _, err := u.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	batches, err := u.repo.List(ctx, domainStockOut.Filter{From: day, To: day.AddDate(0, 0, 1), Limit: 5000})
	if err != nil {
		return nil, err
	}
	out := map[uint]int{}
	pending := 0
	for _, b := range batches {
		if !counted(b) {
			continue
		}
		if b.Status == domainStockOut.StatusReview && b.ReviewedAt == nil {
			pending++
		}
		for _, l := range b.Lines {
			out[l.SKUID] += l.Qty
		}
	}
	sum := &Summary{Date: label, WarehouseID: cfg.WarehouseID, Boxes: []BoxSummary{}, PendingRev: pending, AsOf: u.now()}
	for _, box := range cfg.Boxes {
		row := BoxSummary{Box: box, OutToday: out[box.SKUID]}
		s, err := u.skus.FindByID(ctx, box.SKUID)
		if err != nil {
			return nil, err
		}
		if s == nil {
			row.Missing = true
			sum.Boxes = append(sum.Boxes, row)
			continue
		}
		row.SKUCode = s.SKU
		stk, err := u.stock.GetBySKUID(ctx, box.SKUID, cfg.WarehouseID)
		if err != nil {
			return nil, err
		}
		if stk != nil {
			row.OnHand = stk.AvailableQty
		}
		row.Low = box.LowStock > 0 && row.OnHand <= box.LowStock
		lots, err := u.stock.FindLotsBySKU(ctx, box.SKUID, cfg.WarehouseID)
		if err != nil {
			return nil, err
		}
		for _, lot := range lots {
			if free := lot.Quantity - lot.ReservedQty; free > 0 {
				row.Lot = &ActiveLot{LotID: lot.ID, LotNumber: lot.LotNumber, Expiry: lot.ExpiryDate, Left: free}
				break
			}
		}
		sum.Boxes = append(sum.Boxes, row)
	}
	return sum, nil
}

func (u *usecase) History(ctx context.Context, from, to string) ([]HistoryRow, error) {
	start, _, err := u.dayRange(from)
	if err != nil {
		return nil, err
	}
	end, _, err := u.dayRange(to)
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		start, end = end, start
	}
	if end.Sub(start) > time.Duration(domainStockOut.MaxHistoryDays)*24*time.Hour {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("pick at most %d days", domainStockOut.MaxHistoryDays), 400)
	}
	batches, err := u.repo.List(ctx, domainStockOut.Filter{From: start, To: end.AddDate(0, 0, 1), Limit: 50000})
	if err != nil {
		return nil, err
	}
	type key struct {
		date string
		sku  uint
	}
	sums := map[key]int{}
	for _, b := range batches {
		if !counted(b) {
			continue
		}
		d := b.CountedAt.In(u.loc).Format("2006-01-02")
		for _, l := range b.Lines {
			sums[key{d, l.SKUID}] += l.Qty
		}
	}
	rows := make([]HistoryRow, 0, len(sums))
	for k, q := range sums {
		rows = append(rows, HistoryRow{Date: k.date, SKUID: k.sku, Qty: q})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		return rows[i].SKUID < rows[j].SKUID
	})
	return rows, nil
}

func (u *usecase) countedAt(in *time.Time) time.Time {
	now := u.now()
	if in == nil || in.IsZero() || in.After(now.Add(domainStockOut.ClientClockSkew)) || in.Before(now.Add(-domainStockOut.MaxBackdate)) {
		return now
	}
	return *in
}

func validate(in BatchInput) (BatchInput, map[uint]int, []uint, error) {
	in.ClientID = strings.TrimSpace(in.ClientID)
	if in.ClientID == "" || len(in.ClientID) > domainStockOut.MaxClientID {
		return in, nil, nil, appErrors.NewAppError("VALIDATION_ERROR", "clientId is required (64 characters max)", 400)
	}
	if len(in.Lines) == 0 || len(in.Lines) > domainStockOut.MaxLines {
		return in, nil, nil, appErrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("a batch needs 1–%d lines", domainStockOut.MaxLines), 400)
	}
	qty := map[uint]int{}
	var order []uint
	for _, l := range in.Lines {
		if l.SKUID == 0 || l.Qty <= 0 {
			return in, nil, nil, appErrors.NewAppError("VALIDATION_ERROR", "every line needs skuId and a positive qty", 400)
		}
		if _, ok := qty[l.SKUID]; !ok {
			order = append(order, l.SKUID)
		}
		qty[l.SKUID] += l.Qty
		if qty[l.SKUID] > domainStockOut.MaxQtyPerLine {
			return in, nil, nil, appErrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("at most %d per SKU in one batch", domainStockOut.MaxQtyPerLine), 400)
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	return in, qty, order, nil
}

// CreateBatch records the count and takes it out of the inventory lot by lot,
// earliest expiry first, in one transaction. A client id that was already
// used returns the first batch and takes nothing again. When the stock row or
// the lots hold less than was counted, the batch is still recorded, takes what
// there is and is marked needs_review; stock never goes below zero.
func (u *usecase) CreateBatch(ctx context.Context, in BatchInput) (*BatchResult, error) {
	in, qty, order, err := validate(in)
	if err != nil {
		return nil, err
	}
	if existing, err := u.repo.FindByClientID(ctx, in.ClientID); err != nil {
		return nil, err
	} else if existing != nil {
		return &BatchResult{Batch: existing, Replayed: true}, nil
	}

	var result *BatchResult
	err = u.tx.Transaction(ctx, func(txCtx context.Context) error {
		if existing, err := u.repo.FindByClientID(txCtx, in.ClientID); err != nil {
			return err
		} else if existing != nil {
			result = &BatchResult{Batch: existing, Replayed: true}
			return nil
		}
		cfg, _, err := u.loadConfig(txCtx)
		if err != nil {
			return err
		}
		if !hasRole(cfg.CountRoles, in.Actor.Role) {
			return appErrors.NewAppError("FORBIDDEN", "this role may not count stock out", 403)
		}
		boxes := map[uint]domainStockOut.Box{}
		for _, b := range cfg.Boxes {
			boxes[b.SKUID] = b
		}

		batch := &domainStockOut.Batch{
			ClientID: in.ClientID, WarehouseID: cfg.WarehouseID, UserID: in.Actor.ID, UserName: actorName(in.Actor),
			Status: domainStockOut.StatusPosted, CountedAt: u.countedAt(in.CountedAt), CreatedAt: u.now(),
		}
		type plan struct {
			sku *domainSKU.SKU
			stk *domainStock.Stock
		}
		plans := make([]plan, 0, len(order))
		for _, id := range order {
			box, ok := boxes[id]
			if !ok {
				return appErrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("SKU %d is not a box on the count screen", id), 400)
			}
			s, err := u.skus.FindByID(txCtx, id)
			if err != nil {
				return err
			}
			if s == nil {
				return appErrors.NewAppError("SKU_NOT_FOUND", fmt.Sprintf("SKU %d not found", id), 404)
			}
			stk, err := u.stock.GetBySKUIDForUpdate(txCtx, id, cfg.WarehouseID)
			if err != nil {
				return err
			}
			lots, err := u.stock.GetAvailableLotsForUpdate(txCtx, id, cfg.WarehouseID)
			if err != nil {
				return err
			}
			available := 0
			if stk != nil {
				available = stk.AvailableQty
				if stk.Quantity < available {
					available = stk.Quantity
				}
			}
			want := qty[id]
			canTake := want
			if available < canTake {
				canTake = available
			}
			if canTake < 0 {
				canTake = 0
			}
			takes := takeFEFO(lots, canTake)
			deducted := 0
			for _, t := range takes {
				deducted += t.Qty
			}
			line := domainStockOut.Line{SKUID: id, SKUCode: s.SKU, Name: box.Name, Group: box.Group,
				Qty: want, Deducted: deducted, Short: want - deducted, Lots: takes}
			if line.Short > 0 {
				batch.Status = domainStockOut.StatusReview
			}
			batch.Lines = append(batch.Lines, line)
			batch.TotalQty += want
			plans = append(plans, plan{sku: s, stk: stk})
		}

		if err := u.repo.Create(txCtx, batch); err != nil {
			return err
		}
		batch.BatchNo = fmt.Sprintf("SO-%06d", batch.ID)
		if err := u.repo.SetBatchNo(txCtx, batch.ID, batch.BatchNo); err != nil {
			return err
		}
		for i, line := range batch.Lines {
			if line.Deducted == 0 {
				continue
			}
			p := plans[i]
			if _, err := u.stock.UpdateQuantity(txCtx, p.sku.ID, cfg.WarehouseID, -line.Deducted); err != nil {
				return err
			}
			current := p.stk.Quantity
			for _, t := range line.Lots {
				if _, err := u.stock.DeductLotQuantity(txCtx, t.LotID, t.Qty); err != nil {
					return err
				}
				lotID := t.LotID
				if err := u.stock.CreateMovement(txCtx, &domainStock.StockMovement{
					SKUID: p.sku.ID, SKUCode: p.sku.SKU, WarehouseID: cfg.WarehouseID, StockLotID: &lotID,
					Channel: domainStockOut.Channel, Type: domainStock.MovementOut, Quantity: t.Qty,
					BeforeQty: current, AfterQty: current - t.Qty,
					ReferenceType: domainStockOut.RefTypeOut, ReferenceID: batch.BatchNo,
					Note: truncate(fmt.Sprintf("นับของออก %s โดย %s (Lot: %s)", batch.BatchNo, batch.UserName, t.LotNumber), 255), CreatedAt: u.now(),
				}); err != nil {
					return err
				}
				current -= t.Qty
			}
		}
		result = &BatchResult{Batch: batch}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !result.Replayed {
		u.publish("batch", result.Batch)
		if result.Batch.Status == domainStockOut.StatusReview && u.notifier != nil {
			u.notifier.NotifyReview(*result.Batch)
		}
	}
	return result, nil
}

// takeFEFO takes qty from the earliest-expiring lots first. It never takes
// stock that has no lot: the count must always name the lot.
func takeFEFO(lots []domainStock.StockLot, qty int) []domainStockOut.LotTake {
	out := []domainStockOut.LotTake{}
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
		out = append(out, domainStockOut.LotTake{LotID: lot.ID, LotNumber: lot.LotNumber, Expiry: lot.ExpiryDate, Qty: take})
		qty -= take
	}
	return out
}

// VoidBatch keeps the batch, marks it voided and puts every taken quantity
// back into the same lots. The person who counted it may void it for a short
// while; after that, or for anyone else, the void role is needed.
func (u *usecase) VoidBatch(ctx context.Context, id uint, actor Actor) (*domainStockOut.Batch, error) {
	var out *domainStockOut.Batch
	err := u.tx.Transaction(ctx, func(txCtx context.Context) error {
		b, err := u.repo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if b == nil {
			return appErrors.NewAppError("NOT_FOUND", "batch not found", 404)
		}
		if b.Status == domainStockOut.StatusVoided {
			return appErrors.NewAppError("ALREADY_VOIDED", "this batch is already voided", 409)
		}
		cfg, _, err := u.loadConfig(txCtx)
		if err != nil {
			return err
		}
		own := b.UserID != 0 && b.UserID == actor.ID && u.now().Sub(b.CreatedAt) <= domainStockOut.OwnVoidWindow
		if !own && !hasRole(cfg.VoidRoles, actor.Role) {
			return appErrors.NewAppError("FORBIDDEN", "only the counter (right after saving) or a role with void permission may void this batch", 403)
		}
		lines := append([]domainStockOut.Line(nil), b.Lines...)
		sort.Slice(lines, func(i, j int) bool { return lines[i].SKUID < lines[j].SKUID })
		for _, l := range lines {
			if l.Deducted == 0 {
				continue
			}
			stk, err := u.stock.GetBySKUIDForUpdate(txCtx, l.SKUID, b.WarehouseID)
			if err != nil {
				return err
			}
			current := 0
			if stk != nil {
				current = stk.Quantity
			}
			if _, err := u.stock.UpdateQuantity(txCtx, l.SKUID, b.WarehouseID, l.Deducted); err != nil {
				return err
			}
			for _, t := range l.Lots {
				if _, err := u.stock.DeductLotQuantity(txCtx, t.LotID, -t.Qty); err != nil {
					return err
				}
				lotID := t.LotID
				if err := u.stock.CreateMovement(txCtx, &domainStock.StockMovement{
					SKUID: l.SKUID, SKUCode: l.SKUCode, WarehouseID: b.WarehouseID, StockLotID: &lotID,
					Channel: domainStockOut.Channel, Type: domainStock.MovementIn, Quantity: t.Qty,
					BeforeQty: current, AfterQty: current + t.Qty,
					ReferenceType: domainStockOut.RefTypeVoid, ReferenceID: b.BatchNo,
					Note: truncate(fmt.Sprintf("ยกเลิกรอบนับ %s คืน Lot %s โดย %s", b.BatchNo, t.LotNumber, actorName(actor)), 255), CreatedAt: u.now(),
				}); err != nil {
					return err
				}
				current += t.Qty
			}
		}
		at := u.now()
		by := actorName(actor)
		if err := u.repo.MarkVoided(txCtx, b.ID, by, at); err != nil {
			return err
		}
		b.Status, b.VoidedAt, b.VoidedBy = domainStockOut.StatusVoided, &at, by
		out = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	u.publish("void", out)
	return out, nil
}

// ReviewBatch records that the owner checked a short batch (fixed the stock
// with a count adjustment, or found why the system was off).
func (u *usecase) ReviewBatch(ctx context.Context, id uint, actor Actor, note string) (*domainStockOut.Batch, error) {
	note = strings.TrimSpace(note)
	if len(note) > 255 {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", "note is too long (255 max)", 400)
	}
	var out *domainStockOut.Batch
	err := u.tx.Transaction(ctx, func(txCtx context.Context) error {
		b, err := u.repo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if b == nil {
			return appErrors.NewAppError("NOT_FOUND", "batch not found", 404)
		}
		if b.Status != domainStockOut.StatusReview {
			return appErrors.NewAppError("NOT_IN_REVIEW", "this batch does not need review", 409)
		}
		at := u.now()
		by := actorName(actor)
		if err := u.repo.MarkReviewed(txCtx, b.ID, by, note, at); err != nil {
			return err
		}
		b.ReviewedAt, b.ReviewedBy, b.ReviewNote = &at, by, note
		out = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	u.publish("review", out)
	return out, nil
}

func actorName(a Actor) string {
	if strings.TrimSpace(a.Name) != "" {
		return a.Name
	}
	return a.Email
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	for len(string(r)) > n {
		r = r[:len(r)-1]
	}
	return string(r)
}
