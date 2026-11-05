package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	domainCatalog "chawy-erp-api/internal/domain/catalog"
	usecaseCatalog "chawy-erp-api/internal/usecase/catalog"
	appErrors "chawy-erp-api/pkg/errors"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// respondCatalogError maps catalog usecase failures to the historical HTTP
// responses: DBError keeps the exact composed message/status the inline
// handler used to produce; AppError keeps the historical machine error code
// (PRODUCT_IN_USE, SKU_ALREADY_EXISTS, ACCESSORY_NOT_FOUND, ...). Note: the
// endpoints shipped in earlier phases keep respondAppError below, which
// intentionally ignores the code and stays byte-identical.
func respondCatalogError(c *fiber.Ctx, err error) error {
	var dbe *usecaseCatalog.DBError
	if errors.As(err, &dbe) {
		switch dbe.Status {
		case fiber.StatusBadRequest:
			return response.BadRequest(c, dbe.Message, dbe.Code)
		default:
			return response.InternalServerError(c, dbe.Message)
		}
	}
	var appErr *appErrors.AppError
	if errors.As(err, &appErr) {
		switch appErr.StatusCode {
		case fiber.StatusNotFound:
			return response.NotFound(c, appErr.Message, appErr.Code)
		case fiber.StatusInternalServerError:
			return response.InternalServerError(c, appErr.Message, appErr.Code)
		default:
			return response.BadRequest(c, appErr.Message, appErr.Code)
		}
	}
	return response.InternalServerError(c, err.Error())
}

func (h *WorkspaceHandler) GetProducts(c *fiber.Ctx) error {
	var bodyReq struct {
		Page     int         `json:"page"`
		Limit    int         `json:"limit"`
		Search   string      `json:"search"`
		Type     string      `json:"type"`
		Category string      `json:"category"`
		Status   string      `json:"status"`
		IsActive interface{} `json:"isActive"`
		IsBundle interface{} `json:"isBundle"`
	}
	if c.Method() == fiber.MethodPost {
		_ = c.BodyParser(&bodyReq)
	}

	page := bodyReq.Page
	if page < 1 {
		page, _ = strconv.Atoi(c.Query("page", "1"))
	}
	if page < 1 {
		page = 1
	}

	limit := bodyReq.Limit
	if limit <= 0 {
		limit, _ = strconv.Atoi(c.Query("limit", "50"))
	}
	if limit <= 0 {
		limit = 1000
	} else if limit > 1000 {
		limit = 1000
	}

	search := strings.TrimSpace(bodyReq.Search)
	if search == "" {
		search = strings.TrimSpace(c.Query("search", ""))
	}

	prodType := strings.TrimSpace(bodyReq.Type)
	if prodType == "" {
		prodType = strings.TrimSpace(bodyReq.Category)
	}
	if prodType == "" {
		prodType = strings.TrimSpace(c.Query("type", ""))
	}
	if prodType == "" {
		prodType = strings.TrimSpace(c.Query("category", ""))
	}

	var isActiveQuery string
	if bodyReq.IsActive != nil {
		isActiveQuery = strings.TrimSpace(fmt.Sprintf("%v", bodyReq.IsActive))
	} else if bodyReq.Status != "" {
		isActiveQuery = strings.TrimSpace(bodyReq.Status)
	} else {
		isActiveQuery = strings.TrimSpace(c.Query("isActive", ""))
		if isActiveQuery == "" {
			isActiveQuery = strings.TrimSpace(c.Query("status", ""))
		}
	}

	var isBundleQuery string
	if bodyReq.IsBundle != nil {
		isBundleQuery = strings.TrimSpace(fmt.Sprintf("%v", bodyReq.IsBundle))
	} else {
		isBundleQuery = strings.TrimSpace(c.Query("isBundle", ""))
	}

	// Filtering, page scan and every aggregation (stock sums, used qty,
	// TikTok reserved, bundle availability, receipt stats) run in the catalog
	// usecase; the handler keeps only HTTP input parsing and page clamping.
	records, total, err := h.catalogUsecase.List(c.Context(), domainCatalog.ProductQuery{
		Page:     page,
		Limit:    limit,
		Search:   search,
		ProdType: prodType,
		IsBundle: isBundleQuery,
		IsActive: isActiveQuery,
	})
	if err != nil {
		return respondCatalogError(c, err)
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) GetProductByCode(c *fiber.Ctx) error {
	rec, err := h.catalogUsecase.GetByCode(c.Context(), c.Params("code"))
	if err != nil {
		return respondCatalogError(c, err)
	}
	return response.OK(c, rec)
}

func (h *WorkspaceHandler) CreateProduct(c *fiber.Ctx) error {
	var req usecaseCatalog.CreateInput
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	rec, err := h.catalogUsecase.Create(c.Context(), req)
	if err != nil {
		return respondCatalogError(c, err)
	}
	return response.Created(c, rec, "Product created successfully")
}

// GetProductByID returns a product by numeric record ID.
func (h *WorkspaceHandler) GetProductByID(c *fiber.Ctx) error {
	s, err := h.catalogUsecase.FindByID(c.Context(), c.Params("id"))
	if err != nil {
		return respondCatalogError(c, err)
	}
	return response.OK(c, fiber.Map{"id": s.ID, "sku": s.SKU, "name": s.Name, "type": s.Category, "retailPrice": s.Price, "cost": s.CostPrice, "isBundle": s.IsBundle, "isActive": s.Status == "active", "image": s.Image})
}

// UpdateProductByID updates a product by numeric record ID.
func (h *WorkspaceHandler) UpdateProductByID(c *fiber.Ctx) error {
	var req usecaseCatalog.UpdateInput
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	s, err := h.catalogUsecase.UpdateByID(c.Context(), c.Params("id"), req)
	if err != nil {
		return respondCatalogError(c, err)
	}

	return response.OK(c, fiber.Map{"id": s.ID, "sku": s.SKU, "name": s.Name, "isActive": s.Status == "active"}, "Product updated successfully")
}

// UpdateProductStatusByID toggles a product's status by numeric record ID.
func (h *WorkspaceHandler) UpdateProductStatusByID(c *fiber.Ctx) error {
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	s, err := h.catalogUsecase.SetStatusByID(c.Context(), c.Params("id"), req.Status)
	if err != nil {
		return respondCatalogError(c, err)
	}
	return response.OK(c, fiber.Map{"sku": s.SKU, "status": s.Status}, "Product status updated successfully")
}

// UpdateProductReorderByID updates a product's reorder point by numeric record ID.
func (h *WorkspaceHandler) UpdateProductReorderByID(c *fiber.Ctx) error {
	var req struct {
		Reorder *int `json:"reorder"`
	}
	if err := c.BodyParser(&req); err != nil || req.Reorder == nil {
		return response.BadRequest(c, "Invalid request body: reorder point is required")
	}
	if *req.Reorder < 0 {
		return response.BadRequest(c, "Reorder point must be greater than or equal to 0")
	}
	s, err := h.catalogUsecase.SetReorderByID(c.Context(), c.Params("id"), *req.Reorder)
	if err != nil {
		return respondCatalogError(c, err)
	}
	return response.OK(c, fiber.Map{"id": s.ID, "sku": s.SKU, "reorder": s.ReorderPoint}, "Reorder point updated successfully")
}

// DeleteProductByID deletes a product by numeric record ID (reference-safe).
func (h *WorkspaceHandler) DeleteProductByID(c *fiber.Ctx) error {
	if err := h.catalogUsecase.DeleteByID(c.Context(), c.Params("id")); err != nil {
		return respondCatalogError(c, err)
	}
	return response.OK(c, nil, "Product deleted successfully")
}

// UpdateProductReorder updates a product's reorder point by SKU code.
func (h *WorkspaceHandler) UpdateProductReorder(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.Params("code"))
	if code == "" {
		return response.BadRequest(c, "Product SKU is required")
	}

	var req struct {
		Reorder *int `json:"reorder"`
	}
	if err := c.BodyParser(&req); err != nil || req.Reorder == nil {
		return response.BadRequest(c, "Invalid request body: reorder point is required")
	}
	if *req.Reorder < 0 {
		return response.BadRequest(c, "Reorder point must be greater than or equal to 0")
	}

	s, err := h.catalogUsecase.SetReorder(c.Context(), code, *req.Reorder)
	if err != nil {
		return respondCatalogError(c, err)
	}

	return response.OK(c, fiber.Map{
		"sku":     s.SKU,
		"reorder": s.ReorderPoint,
	}, "Reorder point updated successfully")
}

func (h *WorkspaceHandler) UpdateProductStatus(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.Params("code"))
	if code == "" {
		return response.BadRequest(c, "Product SKU is required")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// FULL-12 v2: the :code param is always a SKU string. Numeric-looking SKUs
	// are matched as SKU, never as record ID — the frontend sends SKU values in
	// the path, so interpreting "42" as an ID mutated/deleted the wrong target.
	// Callers that need ID-based access use /products/id/:id routes.
	s, err := h.catalogUsecase.SetStatus(c.Context(), code, req.Status)
	if err != nil {
		return respondCatalogError(c, err)
	}

	return response.OK(c, fiber.Map{
		"sku":    s.SKU,
		"status": s.Status,
	}, "Product status updated successfully")
}

func (h *WorkspaceHandler) UpdateProduct(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.Params("code"))
	if code == "" {
		return response.BadRequest(c, "Product SKU is required")
	}

	var req usecaseCatalog.UpdateInput
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	rec, err := h.catalogUsecase.Update(c.Context(), code, req)
	if err != nil {
		return respondCatalogError(c, err)
	}
	return response.OK(c, rec, "Product updated successfully")
}

func (h *WorkspaceHandler) DeleteProduct(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.Params("code"))
	if code == "" {
		return response.BadRequest(c, "Product SKU is required")
	}

	if err := h.catalogUsecase.Delete(c.Context(), code); err != nil {
		return respondCatalogError(c, err)
	}
	return response.OK(c, nil, "Product deleted successfully")
}

func (h *WorkspaceHandler) GetSKUAccessories(c *fiber.Ctx) error {
	records, err := h.catalogUsecase.Accessories(c.Context(), c.Params("sku"))
	if err != nil {
		return respondCatalogError(c, err)
	}
	return response.OK(c, records)
}

// DownloadProductTemplate generates and downloads an Excel (.xlsx) template for importing SKUs
func (h *WorkspaceHandler) DownloadProductTemplate(c *fiber.Ctx) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "SKU_Import"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"รหัส SKU (จำเป็น)*",
		"ชื่อสินค้า (จำเป็น)*",
		"บาร์โค้ด",
		"หมวดหมู่",
		"ราคาขาย (บาท)",
		"ราคาต้นทุน (บาท)",
		"จำนวนสต็อกเริ่มต้น",
		"จุดสั่งซื้อซ้ำ (Reorder Point)",
	}

	for colIdx, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f.SetCellValue(sheet, cell, header)
	}

	// Sample rows
	sampleData := [][]interface{}{
		{"SAMPLE-SKU-001", "สินค้าตัวอย่าง 1", "885000000001", "Finished Product", 299.00, 150.00, 50, 10},
		{"SAMPLE-SKU-002", "สินค้าตัวอย่าง 2", "885000000002", "Finished Product", 590.00, 300.00, 20, 5},
	}

	for rowIdx, row := range sampleData {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue(sheet, cell, val)
		}
	}

	// Styling: Header style
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2563EB"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheet, 1, 1, headerStyle)
	}

	// Auto column widths
	f.SetColWidth(sheet, "A", "A", 22)
	f.SetColWidth(sheet, "B", "B", 35)
	f.SetColWidth(sheet, "C", "C", 20)
	f.SetColWidth(sheet, "D", "D", 20)
	f.SetColWidth(sheet, "E", "F", 18)
	f.SetColWidth(sheet, "G", "H", 25)

	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", "attachment; filename=sku_import_template.xlsx")
	return f.Write(c.Response().BodyWriter())
}

// ImportProductsXLSX handles bulk importing SKUs from an uploaded .xlsx file
func (h *WorkspaceHandler) ImportProductsXLSX(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "กรุณาเลือกไฟล์ Excel (.xlsx) ที่ต้องการอัปโหลด")
	}

	// Validate extension
	ext := strings.ToLower(file.Filename)
	if !strings.HasSuffix(ext, ".xlsx") {
		return response.BadRequest(c, "รองรับเฉพาะไฟล์ Excel นามสกุล .xlsx เท่านั้น")
	}

	src, err := file.Open()
	if err != nil {
		return response.BadRequest(c, "ไม่สามารถเปิดไฟล์ Excel ได้: "+err.Error())
	}
	defer src.Close()

	excelFile, err := excelize.OpenReader(src)
	if err != nil {
		return response.BadRequest(c, "โครงสร้างไฟล์ Excel ไม่ถูกต้อง: "+err.Error())
	}
	defer excelFile.Close()

	sheets := excelFile.GetSheetList()
	if len(sheets) == 0 {
		return response.BadRequest(c, "ไม่พบ Sheet ในไฟล์ Excel")
	}

	sheetName := sheets[0]
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		return response.BadRequest(c, "ไม่สามารถอ่านแถวข้อมูลในไฟล์ Excel ได้: "+err.Error())
	}

	if len(rows) <= 1 {
		return response.BadRequest(c, "ไฟล์ไม่มีข้อมูลสำหรับนำเข้า (พบเฉพาะหัวตารางหรือว่างเปล่า)")
	}

	// Check column count
	if len(rows[0]) < 8 {
		return response.BadRequest(c, "ไฟล์ Excel ไม่ถูกต้อง: ต้องมีอย่างน้อย 8 คอลัมน์")
	}

	// Parse every data row; blank rows are skipped the same way they used to
	// decrement TotalRows. All validation, duplicate checks, value parsing and
	// the per-row transactions happen in the catalog usecase.
	totalRows := len(rows) - 1
	lines := make([]usecaseCatalog.ImportLine, 0, totalRows)
	for i := 1; i < len(rows); i++ {
		rowNum := i + 1
		row := rows[i]

		if len(row) == 0 {
			totalRows--
			continue
		}

		getCol := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		skuCode := getCol(0)
		name := getCol(1)
		barcode := getCol(2)
		category := getCol(3)
		priceStr := getCol(4)
		costStr := getCol(5)
		stockStr := getCol(6)
		reorderStr := getCol(7)

		// If entire row is blank, skip
		if skuCode == "" && name == "" && barcode == "" {
			totalRows--
			continue
		}

		lines = append(lines, usecaseCatalog.ImportLine{
			Row:      rowNum,
			SKU:      skuCode,
			Name:     name,
			Barcode:  barcode,
			Category: category,
			Price:    priceStr,
			Cost:     costStr,
			Stock:    stockStr,
			Reorder:  reorderStr,
		})
	}

	summary := h.catalogUsecase.Import(c.Context(), totalRows, lines)

	msg := fmt.Sprintf("นำเข้าข้อมูลสำเร็จ: สร้างใหม่ %d รายการ, อัปเดต %d รายการ", summary.Created, summary.Updated)
	if summary.Failed > 0 {
		msg += fmt.Sprintf(", ไม่สำเร็จ %d รายการ", summary.Failed)
	}

	return response.OK(c, summary, msg)
}

