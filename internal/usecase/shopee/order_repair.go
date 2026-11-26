package shopee

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
)

// Repairing re-reads an Order export and corrects what the old parser stored
// wrongly on orders that already exist (order date, item quantity) and fills in
// the variation name. Rows are updated in place: item IDs, corrected SKUs and
// SKU confirmations stay as they are, and nothing is inserted or deleted.

// RepairSkip is an order in the file that the repair leaves alone, and why.
type RepairSkip struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

// OrderRepairPreview summarises what a repair of one file would change.
type OrderRepairPreview struct {
	// TotalRows is the number of data rows in the file.
	TotalRows       int `json:"total_rows"`
	OrdersInFile    int `json:"orders_in_file"`
	OrdersInDB      int `json:"orders_in_db"`
	OrdersNotInDB   int `json:"orders_not_in_db"`
	OrdersToRepair  int `json:"orders_to_repair"`
	OrdersUnchanged int `json:"orders_unchanged"`
	DateChanges     int `json:"date_changes"`
	// MonthChanges counts orders whose month (Bangkok time) moves.
	MonthChanges     int                        `json:"month_changes"`
	QtyChanges       int                        `json:"qty_changes"`
	VariationChanges int                        `json:"variation_changes"`
	Repairs          []domainShopee.OrderRepair `json:"repairs"`
	Skipped          []RepairSkip               `json:"skipped"`
}

// OrderRepairResult is what an applied repair changed.
type OrderRepairResult struct {
	BatchID        string       `json:"batch_id"`
	OrdersRepaired int          `json:"orders_repaired"`
	ItemsRepaired  int          `json:"items_repaired"`
	Skipped        []RepairSkip `json:"skipped"`
	// InsertedCount / SkippedCount let the multi-file upload dialog show the result.
	InsertedCount int `json:"inserted_count"`
	SkippedCount  int `json:"skipped_count"`
}

func (u *orderUsecase) PreviewOrderRepair(ctx context.Context, reader io.Reader, filename string) (*OrderRepairPreview, error) {
	return u.planRepair(ctx, reader, filename)
}

func (u *orderUsecase) ApplyOrderRepair(ctx context.Context, reader io.Reader, filename, actor string) (*OrderRepairResult, error) {
	// The plan is rebuilt from the current data, and every update is guarded by
	// the old value, so applying never writes over a change made since the preview.
	plan, err := u.planRepair(ctx, reader, filename)
	if err != nil {
		return nil, err
	}
	batchID := newRepairBatchID()
	if err := u.orderRepo.ApplyRepairs(ctx, plan.Repairs, batchID, actor, filename); err != nil {
		return nil, err
	}
	items := 0
	for _, r := range plan.Repairs {
		items += len(r.Items)
	}
	return &OrderRepairResult{
		BatchID:        batchID,
		OrdersRepaired: len(plan.Repairs),
		ItemsRepaired:  items,
		Skipped:        plan.Skipped,
		InsertedCount:  len(plan.Repairs),
		SkippedCount:   len(plan.Skipped),
	}, nil
}

func (u *orderUsecase) planRepair(ctx context.Context, reader io.Reader, filename string) (*OrderRepairPreview, error) {
	grid, err := readGridFromStream(reader, filename)
	if err != nil {
		return nil, err
	}
	parsed, err := parseRawGrid(grid)
	if err != nil {
		return nil, err
	}
	parsed = dedupeParsedRows(parsed)

	fileOrders := map[string][]parsedRow{}
	var ids []string
	for _, r := range parsed {
		if _, ok := fileOrders[r.OrderID]; !ok {
			ids = append(ids, r.OrderID)
		}
		fileOrders[r.OrderID] = append(fileOrders[r.OrderID], r)
	}

	existing, err := u.orderRepo.FindByOrderIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	stored := make(map[string]domainShopee.ShopeeOrder, len(existing))
	for _, o := range existing {
		stored[o.ID] = o
	}

	res := &OrderRepairPreview{TotalRows: len(parsed), OrdersInFile: len(ids), OrdersInDB: len(existing)}
	res.OrdersNotInDB = res.OrdersInFile - res.OrdersInDB
	for _, id := range ids {
		order, ok := stored[id]
		if !ok {
			continue
		}
		rep, skip := planOrderRepair(order, fileOrders[id])
		switch {
		case skip != "":
			res.Skipped = append(res.Skipped, RepairSkip{OrderID: id, Reason: skip})
		case rep == nil:
			res.OrdersUnchanged++
		default:
			res.Repairs = append(res.Repairs, *rep)
			if rep.DateChanged {
				res.DateChanges++
				if rep.OldOrderDate.In(shopeeTZ).Format("2006-01") != rep.NewOrderDate.In(shopeeTZ).Format("2006-01") {
					res.MonthChanges++
				}
			}
			for _, it := range rep.Items {
				if it.OldQty != it.NewQty {
					res.QtyChanges++
				}
				if it.OldVariation != it.NewVariation {
					res.VariationChanges++
				}
			}
		}
	}
	res.OrdersToRepair = len(res.Repairs)
	return res, nil
}

// lineKey identifies an order line by what the export holds and users never
// edit: the SKU as imported and the line total (in satang).
type lineKey struct {
	sku    string
	amount int64
}

func satang(v float64) int64 { return int64(math.Round(v * 100)) }

// planOrderRepair compares one stored order with its rows in the file. It
// returns nil when nothing differs, or a skip reason when the lines cannot be
// paired one to one (then the order is left untouched rather than guessed).
func planOrderRepair(order domainShopee.ShopeeOrder, rows []parsedRow) (*domainShopee.OrderRepair, string) {
	if len(order.Items) != len(rows) {
		return nil, fmt.Sprintf("จำนวนบรรทัดไม่ตรงกัน (ในระบบ %d, ในไฟล์ %d)", len(order.Items), len(rows))
	}

	dbByKey := map[lineKey][]domainShopee.ShopeeOrderItem{}
	for _, it := range order.Items {
		sku := it.OriginalSKU
		if sku == "" {
			sku = it.SKU
		}
		k := lineKey{sku, satang(it.SalePrice)}
		dbByKey[k] = append(dbByKey[k], it)
	}
	for k := range dbByKey {
		items := dbByKey[k]
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}

	rep := &domainShopee.OrderRepair{OrderID: order.ID, OldOrderDate: order.OrderDate, NewOrderDate: rows[0].OrderDate}
	rep.DateChanged = !sameMinute(order.OrderDate, rows[0].OrderDate)

	used := map[lineKey]int{}
	for _, row := range rows {
		k := lineKey{row.SKU, satang(row.SalePrice)}
		candidates := dbByKey[k]
		if used[k] >= len(candidates) {
			return nil, fmt.Sprintf("ไม่พบบรรทัด SKU %q ยอด %.2f ในระบบ", row.SKU, row.SalePrice)
		}
		// Lines with the same SKU and total are interchangeable; pair them in order.
		it := candidates[used[k]]
		used[k]++

		newVariation := it.VariationName
		if row.VariationName != "" {
			newVariation = row.VariationName
		}
		if it.Qty != row.Qty || it.VariationName != newVariation {
			rep.Items = append(rep.Items, domainShopee.OrderItemRepair{
				ItemID:       it.ID,
				SKU:          it.SKU,
				ProductName:  it.ProductName,
				OldQty:       it.Qty,
				NewQty:       row.Qty,
				OldVariation: it.VariationName,
				NewVariation: newVariation,
			})
		}
	}

	if !rep.DateChanged && len(rep.Items) == 0 {
		return nil, ""
	}
	return rep, ""
}

// sameMinute compares two instants at minute precision (exports carry no seconds).
func sameMinute(a, b time.Time) bool {
	return a.Truncate(time.Minute).Equal(b.Truncate(time.Minute))
}

func newRepairBatchID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "repair-" + time.Now().In(shopeeTZ).Format("20060102-150405") + "-" + hex.EncodeToString(b)
}
