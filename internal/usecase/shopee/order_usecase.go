package shopee

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"

	"github.com/xuri/excelize/v2"
)

var (
	ErrHeaderNotFound    = errors.New("cannot find header row with 'หมายเลขคำสั่งซื้อ' or 'Order ID' in first 10 rows")
	ErrUnsupportedFormat = errors.New("unsupported file format, only .xlsx, .xls, .csv allowed")
)

type DuplicateDetail struct {
	Identifier    string `json:"identifier"`     // e.g. Order ID "260301ABC01"
	RowIndex      int    `json:"row_index"`      // แถวที่พบในไฟล์ (ถ้ามี)
	DuplicateType string `json:"duplicate_type"` // "FILE_DUPLICATE" or "DB_EXISTING"
	Message       string `json:"message"`        // ข้อความแจ้งเตือนรายละเอียด
}

type OrderPreviewResult struct {
	TotalRows        int                            `json:"total_rows"`
	TotalOrders      int                            `json:"total_orders"`
	BlankSKUCount    int                            `json:"blank_sku_count"`
	DuplicateCount   int                            `json:"duplicate_count"`
	DuplicateOrders  []string                       `json:"duplicate_orders"`
	DuplicateDetails []DuplicateDetail              `json:"duplicate_details"`
	SampleRows       []domainShopee.ShopeeOrderItem `json:"sample_rows"`
	SampleOrders     []domainShopee.ShopeeOrder     `json:"sample_orders"`
}

type OrderImportResult struct {
	InsertedCount    int               `json:"inserted_count"`
	SkippedCount     int               `json:"skipped_count"`
	TotalRows        int               `json:"total_rows"`
	DuplicateCount   int               `json:"duplicate_count"`
	DuplicateOrders  []string          `json:"duplicate_orders"`
	DuplicateDetails []DuplicateDetail `json:"duplicate_details"`
}

type OrderUsecase interface {
	PreviewOrderFile(ctx context.Context, reader io.Reader, filename string) (*OrderPreviewResult, error)
	ImportOrderFile(ctx context.Context, reader io.Reader, filename string) (*OrderImportResult, error)
	// PreviewOrderRepair shows what re-reading an export would correct on orders already stored.
	PreviewOrderRepair(ctx context.Context, reader io.Reader, filename string) (*OrderRepairPreview, error)
	// ApplyOrderRepair applies that correction in one transaction and logs every change.
	ApplyOrderRepair(ctx context.Context, reader io.Reader, filename, actor string) (*OrderRepairResult, error)
	GetOrders(ctx context.Context, filter domainShopee.OrderFilter) ([]domainShopee.ShopeeOrder, int64, error)
	GetOrderByID(ctx context.Context, id string) (*domainShopee.ShopeeOrder, error)
	GetProvinces(ctx context.Context) ([]string, error)
	UpdateItemSKU(ctx context.Context, itemID uint, newSKU string) error
	DeleteOrder(ctx context.Context, id string) error
}

type orderUsecase struct {
	orderRepo domainShopee.OrderRepository
}

func NewOrderUsecase(orderRepo domainShopee.OrderRepository) OrderUsecase {
	return &orderUsecase{
		orderRepo: orderRepo,
	}
}

type parsedRow struct {
	OrderID       string
	OrderDate     time.Time
	SKU           string
	ProductName   string
	VariationName string
	Qty           int
	SalePrice     float64
	BuyerUsername string
	Province      string
	RawRowHash    string
	LineIndex     int
}

// shopeeTZ is the time zone of the timestamps in Shopee Seller Centre exports
// (they carry no offset). A fixed zone avoids depending on tzdata being installed.
var shopeeTZ = time.FixedZone("ICT", 7*60*60)

// orderColumn says which header holds a field. Exact names are tried first, in
// order; only when none is present are the partial patterns tried. The first
// matching column wins, so a later header that merely contains the same word
// (e.g. "จำนวนเงินทั้งหมด" for "จำนวน", or "เวลาที่ทำการสั่งซื้อสำเร็จ" for
// the order time) can never take the field over.
type orderColumn struct {
	key      string
	exact    []string
	contains []string
}

var orderColumns = []orderColumn{
	{key: "order_id", exact: []string{"หมายเลขคำสั่งซื้อ", "Order ID"}, contains: []string{"หมายเลขคำสั่งซื้อ"}},
	{key: "order_date", exact: []string{"วันที่ทำการสั่งซื้อ", "Order Creation Date", "เวลาที่ทำการสั่งซื้อ"}, contains: []string{"วันที่ทำการสั่งซื้อ", "Order Creation Date"}},
	{key: "sku", exact: []string{"เลขอ้างอิง SKU (SKU Reference No.)", "SKU Reference No.", "เลขอ้างอิง SKU", "SKU"}, contains: []string{"SKU Reference No.", "เลขอ้างอิง SKU", "Parent SKU"}},
	{key: "product_name", exact: []string{"ชื่อสินค้า", "Product Name"}, contains: []string{"ชื่อสินค้า", "Product Name"}},
	{key: "variation_name", exact: []string{"ชื่อตัวเลือก", "Variation Name"}, contains: []string{"ชื่อตัวเลือก", "Variation Name"}},
	{key: "qty", exact: []string{"จำนวน", "Quantity", "Qty"}},
	// "ราคาขายสุทธิ" is the line total after seller discounts (BR-01); the plain
	// "ราคาขาย" header is kept for files that have only that column.
	{key: "sale_price", exact: []string{"ราคาขายสุทธิ", "ราคาขาย", "Deal Price", "ราคาต่อหน่วย", "Unit Price"}, contains: []string{"ราคาขายสุทธิ", "Deal Price"}},
	{key: "buyer_username", exact: []string{"ชื่อผู้ใช้ (ผู้ซื้อ)", "Buyer Username"}, contains: []string{"ชื่อผู้ใช้ (ผู้ซื้อ)", "Buyer Username"}},
	{key: "province", exact: []string{"จังหวัด", "Province", "State"}},
}

// resolveOrderColumns maps each field to its column index in the header row.
func resolveOrderColumns(header []string) map[string]int {
	cells := make([]string, len(header))
	for i, h := range header {
		cells[i] = strings.TrimSpace(h)
	}
	find := func(match func(cell, name string) bool, names []string) (int, bool) {
		for _, name := range names {
			for i, cell := range cells {
				if match(cell, name) {
					return i, true
				}
			}
		}
		return 0, false
	}
	exact := func(cell, name string) bool { return strings.EqualFold(cell, name) }
	partial := func(cell, name string) bool { return strings.Contains(cell, name) }

	colIdx := map[string]int{}
	for _, col := range orderColumns {
		if i, ok := find(exact, col.exact); ok {
			colIdx[col.key] = i
		} else if i, ok := find(partial, col.contains); ok {
			colIdx[col.key] = i
		}
	}
	return colIdx
}

// parseQty reads a quantity cell, which spreadsheets may format as "2" or "2.00".
func parseQty(raw string) int {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return int(f)
	}
	return 0
}

// parseOrderDate parses a date without a zone as UTC. The Income import still
// uses it for transfer dates, whose reports group by the stored UTC date.
func parseOrderDate(raw string) time.Time {
	return parseShopeeTime(raw, time.UTC)
}

// parseOrderTime parses the order time of an Order export, which is Bangkok
// local time, so the stored instant (and the month it falls in) is right.
func parseOrderTime(raw string) time.Time {
	return parseShopeeTime(raw, shopeeTZ)
}

func parseShopeeTime(raw string, loc *time.Location) time.Time {
	raw = strings.TrimSpace(raw)
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"02/01/2006 15:04:05",
		"02/01/2006 15:04",
		"02/01/2006",
		"02-01-2006 15:04:05",
		"02-01-2006 15:04",
		"02-01-2006",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, raw, loc); err == nil {
			return t
		}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	return time.Now()
}

func computeRowHash(orderID, sku string, qty int, price float64, date time.Time) string {
	raw := fmt.Sprintf("%s|%s|%d|%.2f|%s", orderID, sku, qty, price, date.Format("2006-01-02 15:04:05"))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func parseRawGrid(grid [][]string) ([]parsedRow, error) {
	if len(grid) == 0 {
		return nil, errors.New("empty file")
	}

	headerRowIdx := -1
	for i := 0; i < len(grid) && i < 10; i++ {
		for _, cell := range grid[i] {
			norm := strings.TrimSpace(cell)
			if strings.Contains(norm, "หมายเลขคำสั่งซื้อ") || strings.EqualFold(norm, "Order ID") || strings.Contains(norm, "คำสั่งซื้อ") {
				headerRowIdx = i
				break
			}
		}
		if headerRowIdx != -1 {
			break
		}
	}

	if headerRowIdx == -1 {
		return nil, ErrHeaderNotFound
	}

	colIdx := resolveOrderColumns(grid[headerRowIdx])

	if _, ok := colIdx["order_id"]; !ok {
		return nil, ErrHeaderNotFound
	}

	getCell := func(row []string, key string) string {
		if idx, ok := colIdx[key]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	var rows []parsedRow
	for r := headerRowIdx + 1; r < len(grid); r++ {
		row := grid[r]
		orderID := getCell(row, "order_id")
		if orderID == "" {
			continue
		}

		skuCode := getCell(row, "sku")
		prodName := getCell(row, "product_name")
		if prodName == "" {
			prodName = "Shopee Product " + skuCode
		}

		qty := parseQty(getCell(row, "qty"))
		if qty <= 0 {
			qty = 1
		}

		priceStr := strings.ReplaceAll(getCell(row, "sale_price"), ",", "")
		price, _ := strconv.ParseFloat(priceStr, 64)

		orderDate := parseOrderTime(getCell(row, "order_date"))
		buyerUser := getCell(row, "buyer_username")
		province := getCell(row, "province")

		hash := computeRowHash(orderID, skuCode, qty, price, orderDate)

		rows = append(rows, parsedRow{
			OrderID:       orderID,
			OrderDate:     orderDate,
			SKU:           skuCode,
			ProductName:   prodName,
			VariationName: getCell(row, "variation_name"),
			Qty:           qty,
			SalePrice:     price,
			BuyerUsername: buyerUser,
			Province:      province,
			RawRowHash:    hash,
			LineIndex:     r + 1,
		})
	}

	return rows, nil
}

func readGridFromStream(reader io.Reader, filename string) ([][]string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	buf, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if ext == ".csv" {
		r := csv.NewReader(bytes.NewReader(buf))
		r.LazyQuotes = true
		r.FieldsPerRecord = -1
		return r.ReadAll()
	} else if ext == ".xlsx" || ext == ".xls" {
		f, err := excelize.OpenReader(bytes.NewReader(buf))
		if err != nil {
			return nil, err
		}
		defer f.Close()

		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, errors.New("no sheets found in Excel file")
		}
		return f.GetRows(sheets[0])
	}

	return nil, ErrUnsupportedFormat
}

func groupRowsToOrders(parsed []parsedRow) []domainShopee.ShopeeOrder {
	orderMap := make(map[string]*domainShopee.ShopeeOrder)
	var orderList []domainShopee.ShopeeOrder

	for _, row := range parsed {
		ord, exists := orderMap[row.OrderID]
		if !exists {
			ord = &domainShopee.ShopeeOrder{
				ID:            row.OrderID,
				OrderDate:     row.OrderDate,
				BuyerUsername: row.BuyerUsername,
				Province:      row.Province,
				Status:        "COMPLETED",
				RowHash:       row.RawRowHash,
				Items:         []domainShopee.ShopeeOrderItem{},
			}
			orderMap[row.OrderID] = ord
		}

		ord.Items = append(ord.Items, domainShopee.ShopeeOrderItem{
			OrderID:       row.OrderID,
			SKU:           row.SKU,
			ProductName:   row.ProductName,
			VariationName: row.VariationName,
			Qty:           row.Qty,
			SalePrice:     row.SalePrice, // BR-01 line total
			OriginalSKU:   row.SKU,
			SKUConfirmed:  false,
		})
	}

	for _, ord := range orderMap {
		orderList = append(orderList, *ord)
	}

	return orderList
}

// dedupeParsedRows keeps the first occurrence of each row hash, dropping rows
// that are exact duplicates of an earlier row in the same file.
func dedupeParsedRows(parsed []parsedRow) []parsedRow {
	seen := make(map[string]bool, len(parsed))
	out := make([]parsedRow, 0, len(parsed))
	for _, r := range parsed {
		if seen[r.RawRowHash] {
			continue
		}
		seen[r.RawRowHash] = true
		out = append(out, r)
	}
	return out
}

func detectOrderDuplicates(ctx context.Context, orderRepo domainShopee.OrderRepository, parsed []parsedRow, orders []domainShopee.ShopeeOrder) (int, []string, []DuplicateDetail) {
	var details []DuplicateDetail
	dupOrderMap := make(map[string]bool)

	// 1. Check internal duplicates in file
	seenRowHashes := make(map[string]int)
	for _, r := range parsed {
		if firstLine, exists := seenRowHashes[r.RawRowHash]; exists {
			dupOrderMap[r.OrderID] = true
			details = append(details, DuplicateDetail{
				Identifier:    r.OrderID,
				RowIndex:      r.LineIndex,
				DuplicateType: "FILE_DUPLICATE",
				Message:       fmt.Sprintf("คำสั่งซื้อ %s (SKU: %s) ซ้ำกับรายการในแถวที่ %d ในไฟล์เดียวกัน", r.OrderID, r.SKU, firstLine),
			})
		} else {
			seenRowHashes[r.RawRowHash] = r.LineIndex
		}
	}

	// 2. Check existing orders in DB
	if orderRepo != nil && len(orders) > 0 {
		orderIDs := make([]string, len(orders))
		for i, o := range orders {
			orderIDs[i] = o.ID
		}

		existingOrders, err := orderRepo.FindByOrderIDs(ctx, orderIDs)
		if err == nil && len(existingOrders) > 0 {
			for _, exist := range existingOrders {
				dupOrderMap[exist.ID] = true
				details = append(details, DuplicateDetail{
					Identifier:    exist.ID,
					RowIndex:      0,
					DuplicateType: "DB_EXISTING",
					Message:       fmt.Sprintf("คำสั่งซื้อ %s มีอยู่ในฐานข้อมูลแล้ว (จะถูกข้ามเพื่อป้องกันข้อมูลซ้ำซ้อน)", exist.ID),
				})
			}
		}
	}

	var duplicateOrders []string
	for id := range dupOrderMap {
		duplicateOrders = append(duplicateOrders, id)
	}

	return len(details), duplicateOrders, details
}

func (u *orderUsecase) PreviewOrderFile(ctx context.Context, reader io.Reader, filename string) (*OrderPreviewResult, error) {
	grid, err := readGridFromStream(reader, filename)
	if err != nil {
		return nil, err
	}

	parsed, err := parseRawGrid(grid)
	if err != nil {
		return nil, err
	}

	blankCount := 0
	for _, p := range parsed {
		if p.SKU == "" {
			blankCount++
		}
	}

	orders := groupRowsToOrders(parsed)
	dupCount, dupOrders, dupDetails := detectOrderDuplicates(ctx, u.orderRepo, parsed, orders)

	sampleLimit := 50
	if len(parsed) < sampleLimit {
		sampleLimit = len(parsed)
	}

	var sampleRows []domainShopee.ShopeeOrderItem
	for i := 0; i < sampleLimit; i++ {
		sampleRows = append(sampleRows, domainShopee.ShopeeOrderItem{
			OrderID:       parsed[i].OrderID,
			SKU:           parsed[i].SKU,
			ProductName:   parsed[i].ProductName,
			VariationName: parsed[i].VariationName,
			Qty:           parsed[i].Qty,
			SalePrice:     parsed[i].SalePrice,
		})
	}

	sampleOrderLimit := 10
	if len(orders) < sampleOrderLimit {
		sampleOrderLimit = len(orders)
	}

	return &OrderPreviewResult{
		TotalRows:        len(parsed),
		TotalOrders:      len(orders),
		BlankSKUCount:    blankCount,
		DuplicateCount:   dupCount,
		DuplicateOrders:  dupOrders,
		DuplicateDetails: dupDetails,
		SampleRows:       sampleRows,
		SampleOrders:     orders[:sampleOrderLimit],
	}, nil
}

func (u *orderUsecase) ImportOrderFile(ctx context.Context, reader io.Reader, filename string) (*OrderImportResult, error) {
	grid, err := readGridFromStream(reader, filename)
	if err != nil {
		return nil, err
	}

	parsed, err := parseRawGrid(grid)
	if err != nil {
		return nil, err
	}

	orders := groupRowsToOrders(parsed)
	dupCount, dupOrders, dupDetails := detectOrderDuplicates(ctx, u.orderRepo, parsed, orders)

	// Skip rows duplicated within the file; rows already in the DB are skipped by BulkInsert.
	unique := dedupeParsedRows(parsed)
	inserted, skipped, err := u.orderRepo.BulkInsert(ctx, groupRowsToOrders(unique))
	if err != nil {
		return nil, err
	}

	return &OrderImportResult{
		InsertedCount:    inserted,
		SkippedCount:     skipped,
		TotalRows:        len(parsed),
		DuplicateCount:   dupCount,
		DuplicateOrders:  dupOrders,
		DuplicateDetails: dupDetails,
	}, nil
}

func (u *orderUsecase) GetOrders(ctx context.Context, filter domainShopee.OrderFilter) ([]domainShopee.ShopeeOrder, int64, error) {
	return u.orderRepo.FindAll(ctx, filter)
}

func (u *orderUsecase) GetOrderByID(ctx context.Context, id string) (*domainShopee.ShopeeOrder, error) {
	return u.orderRepo.FindByID(ctx, id)
}

func (u *orderUsecase) GetProvinces(ctx context.Context) ([]string, error) {
	return u.orderRepo.GetDistinctProvinces(ctx)
}

func (u *orderUsecase) UpdateItemSKU(ctx context.Context, itemID uint, newSKU string) error {
	newSKU = strings.TrimSpace(newSKU)
	if newSKU == "" {
		return errors.New("SKU cannot be empty")
	}
	return u.orderRepo.UpdateItemSKU(ctx, itemID, newSKU, true)
}

func (u *orderUsecase) DeleteOrder(ctx context.Context, id string) error {
	return u.orderRepo.Delete(ctx, id)
}
