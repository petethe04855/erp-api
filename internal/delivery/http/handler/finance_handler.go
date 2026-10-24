package handler

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chawy-erp-api/internal/delivery/http/dto"
	domainFinance "chawy-erp-api/internal/domain/finance"
	usecaseFinance "chawy-erp-api/internal/usecase/finance"
	"chawy-erp-api/pkg/gemini"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type FinanceHandler struct {
	usecase   usecaseFinance.Usecase
	geminiKey string
	scanner   *gemini.Scanner
}

func NewFinanceHandler(usecase usecaseFinance.Usecase, geminiKey ...string) *FinanceHandler {
	key := ""
	if len(geminiKey) > 0 {
		key = geminiKey[0]
	}
	return &FinanceHandler{
		usecase:   usecase,
		geminiKey: key,
		scanner:   gemini.NewScanner(),
	}
}

// Journal Entries
func (h *FinanceHandler) ListJournalEntries(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	from := c.Query("from", "")
	to := c.Query("to", "")
	sourceType := c.Query("source_type", "")
	search := c.Query("search", "")

	entries, total, err := h.usecase.ListJournalEntries(c.Context(), domainFinance.JournalFilter{
		From:       from,
		To:         to,
		SourceType: sourceType,
		Search:     search,
		Page:       page,
		Limit:      limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, entries, page, limit, total)
}

func (h *FinanceHandler) GetJournalEntryByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid journal entry ID")
	}

	entry, err := h.usecase.GetJournalEntryByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, entry)
}

// Expenses
func (h *FinanceHandler) ListExpenses(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	from := c.Query("from", "")
	to := c.Query("to", "")
	category := c.Query("category", "")
	channel := c.Query("channel", "")
	search := c.Query("search", "")

	expenses, total, err := h.usecase.ListExpenses(c.Context(), domainFinance.ExpenseFilter{
		From:     from,
		To:       to,
		Category: category,
		Channel:  channel,
		Search:   search,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, expenses, page, limit, total)
}

func (h *FinanceHandler) CreateExpense(c *fiber.Ctx) error {
	var req dto.CreateExpenseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	username, _ := c.Locals("email").(string)
	if username == "" {
		username = "system"
	}

	exp, err := h.usecase.CreateExpense(c.Context(), usecaseFinance.CreateExpenseInput{
		Date:        req.Date,
		Category:    domainFinance.ExpenseCategory(req.Category),
		Channel:     domainFinance.ExpenseChannel(req.Channel),
		Amount:      req.Amount,
		ProductName: req.ProductName,
		PurchaseURL: req.PurchaseURL,
		SlipURL:     req.SlipURL,
		Vendor:      req.Vendor,
		InvoiceRef:  req.InvoiceRef,
		Description: req.Description,
		CreatedBy:   username,
	})
	if err != nil {
		return err
	}

	return response.Created(c, exp, "Expense created successfully")
}

func (h *FinanceHandler) UpdateExpense(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid expense ID")
	}

	var req dto.UpdateExpenseRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	exp, err := h.usecase.UpdateExpense(c.Context(), uint(id), usecaseFinance.UpdateExpenseInput{
		Date:        req.Date,
		Category:    domainFinance.ExpenseCategory(req.Category),
		Channel:     domainFinance.ExpenseChannel(req.Channel),
		Amount:      req.Amount,
		ProductName: req.ProductName,
		PurchaseURL: req.PurchaseURL,
		SlipURL:     req.SlipURL,
		Vendor:      req.Vendor,
		InvoiceRef:  req.InvoiceRef,
		Description: req.Description,
	})
	if err != nil {
		return err
	}

	return response.OK(c, exp, "Expense updated successfully")
}

func (h *FinanceHandler) DeleteExpense(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid expense ID")
	}

	if err := h.usecase.DeleteExpense(c.Context(), uint(id)); err != nil {
		return err
	}

	return response.OK(c, fiber.Map{"id": id}, "Expense deleted successfully")
}

// ScanReceipt handles uploading receipt/slip image and running Gemini Vision OCR.
func (h *FinanceHandler) ScanReceipt(c *fiber.Ctx) error {
	file, err := c.FormFile("image")
	if err != nil {
		file, err = c.FormFile("slip")
		if err != nil {
			file, err = c.FormFile("file")
			if err != nil {
				return response.BadRequest(c, "กรุณาแนบไฟล์รูปภาพสลิปหรือใบเสร็จ (image หรือ slip)")
			}
		}
	}

	const maxFileSize = 10 * 1024 * 1024 // 10MB
	if file.Size > maxFileSize {
		return response.BadRequest(c, "ขนาดไฟล์รูปภาพเกินกำหนด (ต้องไม่เกิน 10 MB)")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return response.BadRequest(c, "รองรับเฉพาะไฟล์รูปภาพ .png, .jpg, .jpeg หรือ .webp เท่านั้น")
	}

	f, err := file.Open()
	if err != nil {
		return response.BadRequest(c, "ไม่สามารถเปิดไฟล์รูปภาพได้")
	}
	defer f.Close()

	fileBytes, err := io.ReadAll(f)
	if err != nil {
		return response.BadRequest(c, "ไม่สามารถอ่านข้อมูลไฟล์รูปภาพได้")
	}

	// Save file locally to ./uploads/expenses
	targetDir := filepath.Join(".", "uploads", "expenses")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return response.InternalServerError(c, "ไม่สามารถสร้างโฟลเดอร์สำหรับจัดเก็บรูปภาพได้")
	}

	filename := fmt.Sprintf("expense-%d-%s%s", time.Now().UnixNano(), uuid.New().String()[:8], ext)
	savedPath := filepath.Join(targetDir, filename)
	if err := os.WriteFile(savedPath, fileBytes, 0644); err != nil {
		return response.InternalServerError(c, "ไม่สามารถบันทึกไฟล์รูปภาพได้")
	}

	slipURL := fmt.Sprintf("/uploads/expenses/%s", filename)

	if strings.TrimSpace(h.geminiKey) == "" {
		return response.BadRequest(c, "กรุณาตั้งค่า GEMINI_API_KEY ในไฟล์ .env ของระบบหลังบ้านเพื่อเปิดใช้งาน AI สแกนใบเสร็จ")
	}

	mimeType := "image/jpeg"
	if ext == ".png" {
		mimeType = "image/png"
	} else if ext == ".webp" {
		mimeType = "image/webp"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scanResult, err := h.scanner.ScanReceipt(ctx, h.geminiKey, fileBytes, mimeType)
	if err != nil {
		return response.BadRequest(c, fmt.Sprintf("AI สแกนรูปภาพไม่สำเร็จ: %v", err))
	}

	type ScanResponse struct {
		*gemini.ReceiptData
		SlipURL string `json:"slip_url"`
	}

	return response.OK(c, ScanResponse{
		ReceiptData: scanResult,
		SlipURL:     slipURL,
	}, "วิเคราะห์ข้อมูลจากใบเสร็จสำเร็จ")
}

// Reports
func (h *FinanceHandler) GetGeneralLedger(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	accountID, _ := strconv.ParseUint(c.Query("account_id", "0"), 10, 32)

	rows, err := h.usecase.GetGeneralLedger(c.Context(), from, to, uint(accountID))
	if err != nil {
		return err
	}

	return response.OK(c, rows)
}

func (h *FinanceHandler) GetTrialBalance(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")

	rows, err := h.usecase.GetTrialBalance(c.Context(), from, to)
	if err != nil {
		return err
	}

	return response.OK(c, rows)
}

func (h *FinanceHandler) GetProfitAndLoss(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	channel := c.Query("channel", "")

	report, err := h.usecase.GetProfitAndLoss(c.Context(), from, to, channel)
	if err != nil {
		return err
	}

	return response.OK(c, report)
}

func (h *FinanceHandler) GetRevenueByChannel(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")

	report, err := h.usecase.GetRevenueByChannel(c.Context(), from, to)
	if err != nil {
		return err
	}

	return response.OK(c, report)
}

// Helper to stream excel file
func streamExcelFile(c *fiber.Ctx, filename string, f *excelize.File) error {
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	return f.Write(c.Response().BodyWriter())
}

// Exports (.xlsx)
func (h *FinanceHandler) ExportJournal(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	sourceType := c.Query("source_type", "")

	entries, _, err := h.usecase.ListJournalEntries(c.Context(), domainFinance.JournalFilter{
		From:       from,
		To:         to,
		SourceType: sourceType,
		Limit:      5000,
	})
	if err != nil {
		return err
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Journal Entries"
	f.SetSheetName("Sheet1", sheet)

	headers := []interface{}{"Date", "Code", "Source Type", "Source Ref", "Description", "Account Code", "Account Name", "Debit", "Credit", "Channel", "Status"}
	f.SetSheetRow(sheet, "A1", &headers)

	rowIdx := 2
	for _, entry := range entries {
		for _, line := range entry.Lines {
			row := []interface{}{
				entry.Date,
				entry.Code,
				entry.SourceType,
				entry.SourceRef,
				entry.Description,
				line.AccountCode,
				line.AccountName,
				line.Debit,
				line.Credit,
				line.Channel,
				string(entry.Status),
			}
			cell := fmt.Sprintf("A%d", rowIdx)
			f.SetSheetRow(sheet, cell, &row)
			rowIdx++
		}
	}

	filename := fmt.Sprintf("journal-export-%s.xlsx", time.Now().Format("2006-01-02"))
	return streamExcelFile(c, filename, f)
}

func (h *FinanceHandler) ExportExpenses(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	category := c.Query("category", "")
	channel := c.Query("channel", "")

	expenses, _, err := h.usecase.ListExpenses(c.Context(), domainFinance.ExpenseFilter{
		From:     from,
		To:       to,
		Category: category,
		Channel:  channel,
		Limit:    5000,
	})
	if err != nil {
		return err
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Expenses"
	f.SetSheetName("Sheet1", sheet)

	headers := []interface{}{"Date", "Code", "Product", "Category", "Channel", "Amount", "Purchase URL", "Slip URL", "Vendor", "Invoice Ref", "Description", "Created By"}
	f.SetSheetRow(sheet, "A1", &headers)

	rowIdx := 2
	for _, exp := range expenses {
		row := []interface{}{
			exp.Date,
			exp.Code,
			exp.ProductName,
			string(exp.Category),
			string(exp.Channel),
			exp.Amount,
			exp.PurchaseURL,
			exp.SlipURL,
			exp.Vendor,
			exp.InvoiceRef,
			exp.Description,
			exp.CreatedBy,
		}
		cell := fmt.Sprintf("A%d", rowIdx)
		f.SetSheetRow(sheet, cell, &row)
		rowIdx++
	}

	filename := fmt.Sprintf("expenses-export-%s.xlsx", time.Now().Format("2006-01-02"))
	return streamExcelFile(c, filename, f)
}

func (h *FinanceHandler) ExportPnL(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	channel := c.Query("channel", "")

	pnl, err := h.usecase.GetProfitAndLoss(c.Context(), from, to, channel)
	if err != nil {
		return err
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Profit & Loss"
	f.SetSheetName("Sheet1", sheet)

	f.SetSheetRow(sheet, "A1", &[]interface{}{"Profit & Loss Report"})
	f.SetSheetRow(sheet, "A2", &[]interface{}{fmt.Sprintf("Period: %s to %s", from, to)})
	f.SetSheetRow(sheet, "A4", &[]interface{}{"Item", "Amount (THB)"})
	f.SetSheetRow(sheet, "A5", &[]interface{}{"Total Revenue", pnl.Revenue})
	f.SetSheetRow(sheet, "A6", &[]interface{}{"Cost of Goods Sold (COGS)", pnl.COGS})
	f.SetSheetRow(sheet, "A7", &[]interface{}{"Gross Profit", pnl.GrossProfit})
	f.SetSheetRow(sheet, "A8", &[]interface{}{"Operating Expenses", pnl.OperatingExpense})
	f.SetSheetRow(sheet, "A9", &[]interface{}{"Net Profit", pnl.NetProfit})

	filename := fmt.Sprintf("pnl-export-%s.xlsx", time.Now().Format("2006-01-02"))
	return streamExcelFile(c, filename, f)
}
