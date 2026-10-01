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
	ErrIncomeSheetNotFound = errors.New("income sheet not found")
)

type IncomePreviewResult struct {
	TotalRows       int                         `json:"total_rows"`
	TotalNetAmount  float64                     `json:"total_net_amount"`
	SampleRows      []domainShopee.ShopeeIncome `json:"sample_rows"`
}

type IncomeImportResult struct {
	InsertedCount int     `json:"inserted_count"`
	SkippedCount  int     `json:"skipped_count"`
	TotalRows     int     `json:"total_rows"`
	TotalAmount   float64 `json:"total_amount"`
}

type IncomeListItem struct {
	domainShopee.ShopeeIncome
	IsMatched bool `json:"is_matched"`
}

type IncomeUsecase interface {
	PreviewIncomeFile(ctx context.Context, reader io.Reader, filename string) (*IncomePreviewResult, error)
	ImportIncomeFile(ctx context.Context, reader io.Reader, filename string) (*IncomeImportResult, error)
	GetIncomes(ctx context.Context, filter domainShopee.IncomeFilter) ([]IncomeListItem, int64, error)
	DeleteIncome(ctx context.Context, id uint) error
}

type incomeUsecase struct {
	incomeRepo domainShopee.IncomeRepository
}

func NewIncomeUsecase(incomeRepo domainShopee.IncomeRepository) IncomeUsecase {
	return &incomeUsecase{
		incomeRepo: incomeRepo,
	}
}

func computeIncomeRowHash(orderID string, transferDate time.Time, netAmount float64) string {
	raw := fmt.Sprintf("%s|%s|%.2f", orderID, transferDate.Format("2006-01-02 15:04:05"), netAmount)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func readIncomeGrid(reader io.Reader, filename string) ([][]string, error) {
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
		hasIncomeSheet := false
		var exactIncomeSheet string
		for _, s := range sheets {
			if strings.EqualFold(strings.TrimSpace(s), "Income") {
				hasIncomeSheet = true
				exactIncomeSheet = s
				break
			}
		}

		if !hasIncomeSheet {
			return nil, fmt.Errorf("%w: excel file must contain a sheet named 'Income', found sheets: %v", ErrIncomeSheetNotFound, sheets)
		}

		return f.GetRows(exactIncomeSheet)
	}

	return nil, ErrUnsupportedFormat
}

func parseIncomeGrid(grid [][]string) ([]domainShopee.ShopeeIncome, error) {
	if len(grid) == 0 {
		return nil, errors.New("empty income file")
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
		} else if strings.Contains(c, "วันที่ทำการสั่งซื้อ") || strings.Contains(c, "Order Creation Date") {
			colIdx["order_date"] = idx
		} else if strings.Contains(c, "วันที่โอนชำระเงินสำเร็จ") || strings.Contains(c, "วันที่โอนเงิน") || strings.Contains(c, "Payout Completed Date") || strings.Contains(c, "Transfer Date") {
			colIdx["transfer_date"] = idx
		} else if strings.Contains(c, "จำนวนเงินทั้งหมดที่โอนแล้ว") || strings.Contains(c, "จำนวนเงินที่โอน") || strings.Contains(c, "Total Released Amount") || strings.Contains(c, "Net Amount") {
			colIdx["net_amount"] = idx
		}
	}

	if _, ok := colIdx["order_id"]; !ok {
		return nil, ErrHeaderNotFound
	}
	if _, ok := colIdx["transfer_date"]; !ok {
		return nil, errors.New("transfer date column not found in income file")
	}

	getCell := func(row []string, key string) string {
		if idx, ok := colIdx[key]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	var records []domainShopee.ShopeeIncome
	for r := headerRowIdx + 1; r < len(grid); r++ {
		row := grid[r]
		orderID := getCell(row, "order_id")
		if orderID == "" {
			continue
		}

		transferDateStr := getCell(row, "transfer_date")
		if transferDateStr == "" {
			continue
		}
		transferDate := parseOrderDate(transferDateStr)

		var orderDate *time.Time
		if odStr := getCell(row, "order_date"); odStr != "" {
			t := parseOrderDate(odStr)
			orderDate = &t
		}

		amountStr := strings.ReplaceAll(getCell(row, "net_amount"), ",", "")
		netAmount, _ := strconv.ParseFloat(amountStr, 64)

		hash := computeIncomeRowHash(orderID, transferDate, netAmount)

		records = append(records, domainShopee.ShopeeIncome{
			OrderID:      orderID,
			OrderDate:    orderDate,
			TransferDate: transferDate,
			NetAmount:    netAmount,
			RowHash:      hash,
		})
	}

	return records, nil
}

func (u *incomeUsecase) PreviewIncomeFile(ctx context.Context, reader io.Reader, filename string) (*IncomePreviewResult, error) {
	grid, err := readIncomeGrid(reader, filename)
	if err != nil {
		return nil, err
	}

	records, err := parseIncomeGrid(grid)
	if err != nil {
		return nil, err
	}

	var totalAmount float64
	for _, r := range records {
		totalAmount += r.NetAmount
	}

	sampleLimit := 50
	if len(records) < sampleLimit {
		sampleLimit = len(records)
	}

	return &IncomePreviewResult{
		TotalRows:      len(records),
		TotalNetAmount: totalAmount,
		SampleRows:     records[:sampleLimit],
	}, nil
}

func (u *incomeUsecase) ImportIncomeFile(ctx context.Context, reader io.Reader, filename string) (*IncomeImportResult, error) {
	grid, err := readIncomeGrid(reader, filename)
	if err != nil {
		return nil, err
	}

	records, err := parseIncomeGrid(grid)
	if err != nil {
		return nil, err
	}

	var totalAmount float64
	for _, r := range records {
		totalAmount += r.NetAmount
	}

	inserted, skipped, err := u.incomeRepo.BulkInsert(ctx, records)
	if err != nil {
		return nil, err
	}

	return &IncomeImportResult{
		InsertedCount: inserted,
		SkippedCount:  skipped,
		TotalRows:     len(records),
		TotalAmount:   totalAmount,
	}, nil
}

func (u *incomeUsecase) GetIncomes(ctx context.Context, filter domainShopee.IncomeFilter) ([]IncomeListItem, int64, error) {
	incomes, total, err := u.incomeRepo.FindAll(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	orderIDs := make([]string, len(incomes))
	for i, inc := range incomes {
		orderIDs[i] = inc.OrderID
	}

	matchedMap, err := u.incomeRepo.GetMatchedOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, 0, err
	}

	items := make([]IncomeListItem, len(incomes))
	for i, inc := range incomes {
		items[i] = IncomeListItem{
			ShopeeIncome: inc,
			IsMatched:    matchedMap[inc.OrderID],
		}
	}

	return items, total, nil
}

func (u *incomeUsecase) DeleteIncome(ctx context.Context, id uint) error {
	return u.incomeRepo.Delete(ctx, id)
}
