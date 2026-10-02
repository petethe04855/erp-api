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
	Identifier    string `json:"identifier"`      // e.g. Order ID "260301ABC01"
	RowIndex      int    `json:"row_index"`       // แถวที่พบในไฟล์ (ถ้ามี)
	DuplicateType string `json:"duplicate_type"`  // "FILE_DUPLICATE" or "DB_EXISTING"
	Message       string `json:"message"`         // ข้อความแจ้งเตือนรายละเอียด
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
	Qty           int
	SalePrice     float64
	BuyerUsername string
	Province      string
	RawRowHash    string
	LineIndex     int
}

func parseOrderDate(raw string) time.Time {
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
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t
		}
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

	header := grid[headerRowIdx]
	colIdx := map[string]int{}
	for idx, col := range header {
		c := strings.TrimSpace(col)
		if strings.Contains(c, "หมายเลขคำสั่งซื้อ") || strings.EqualFold(c, "Order ID") {
			colIdx["order_id"] = idx
		} else if strings.Contains(c, "วันที่ทำการสั่งซื้อ") || strings.Contains(c, "เวลาที่ทำการสั่งซื้อ") || strings.Contains(c, "Order Creation Date") {
			colIdx["order_date"] = idx
		} else if strings.Contains(c, "เลขอ้างอิง SKU") || strings.Contains(c, "SKU Reference No.") || strings.Contains(c, "Parent SKU") || strings.EqualFold(c, "SKU") {
			colIdx["sku"] = idx
		} else if strings.Contains(c, "ชื่อสินค้า") || strings.Contains(c, "Product Name") {
			colIdx["product_name"] = idx
		} else if strings.Contains(c, "จำนวน") || strings.EqualFold(c, "Quantity") || strings.EqualFold(c, "Qty") {
			colIdx["qty"] = idx
		} else if strings.Contains(c, "ราคาขาย") || strings.Contains(c, "Deal Price") || strings.Contains(c, "ราคาต่อหน่วย") || strings.Contains(c, "Unit Price") {
			colIdx["sale_price"] = idx
		} else if strings.Contains(c, "ชื่อผู้ใช้ (ผู้ซื้อ)") || strings.Contains(c, "Buyer Username") {
			colIdx["buyer_username"] = idx
		} else if strings.Contains(c, "จังหวัด") || strings.EqualFold(c, "Province") || strings.EqualFold(c, "State") {
			colIdx["province"] = idx
		}
	}

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

		qtyStr := getCell(row, "qty")
		qty, _ := strconv.Atoi(qtyStr)
		if qty <= 0 {
			qty = 1
		}

		priceStr := strings.ReplaceAll(getCell(row, "sale_price"), ",", "")
		price, _ := strconv.ParseFloat(priceStr, 64)

		orderDate := parseOrderDate(getCell(row, "order_date"))
		buyerUser := getCell(row, "buyer_username")
		province := getCell(row, "province")

		hash := computeRowHash(orderID, skuCode, qty, price, orderDate)

		rows = append(rows, parsedRow{
			OrderID:       orderID,
			OrderDate:     orderDate,
			SKU:           skuCode,
			ProductName:   prodName,
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
			OrderID:      row.OrderID,
			SKU:          row.SKU,
			ProductName:  row.ProductName,
			Qty:          row.Qty,
			SalePrice:    row.SalePrice, // BR-01 line total
			OriginalSKU:  row.SKU,
			SKUConfirmed: false,
		})
	}

	for _, ord := range orderMap {
		orderList = append(orderList, *ord)
	}

	return orderList
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
			OrderID:     parsed[i].OrderID,
			SKU:         parsed[i].SKU,
			ProductName: parsed[i].ProductName,
			Qty:         parsed[i].Qty,
			SalePrice:   parsed[i].SalePrice,
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

	inserted, skipped, err := u.orderRepo.BulkInsert(ctx, orders)
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
