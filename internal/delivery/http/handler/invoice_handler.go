package handler

import (
	"fmt"
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	domainCustomer "chawy-erp-api/internal/domain/customer"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	usecaseInvoice "chawy-erp-api/internal/usecase/invoice"
	usecaseSettings "chawy-erp-api/internal/usecase/settings"
	"chawy-erp-api/pkg/pdf"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type InvoiceHandler struct {
	usecase         usecaseInvoice.Usecase
	db              *gorm.DB
	settingsUsecase usecaseSettings.Usecase
}

func NewInvoiceHandler(usecase usecaseInvoice.Usecase, db *gorm.DB, settingsUsecase usecaseSettings.Usecase) *InvoiceHandler {
	return &InvoiceHandler{
		usecase:         usecase,
		db:              db,
		settingsUsecase: settingsUsecase,
	}
}

func (h *InvoiceHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateInvoiceRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.OrderID == 0 {
		return response.BadRequest(c, "Order ID is required")
	}

	result, err := h.usecase.CreateFromOrder(c.Context(), usecaseInvoice.CreateInvoiceFromOrderInput{
		OrderID: req.OrderID,
	})
	if err != nil {
		return err
	}

	return response.Created(c, result, "Invoice created successfully")
}

func (h *InvoiceHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid invoice ID")
	}

	result, err := h.usecase.GetByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, result)
}

func (h *InvoiceHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")
	status := c.Query("status", "")

	items, total, err := h.usecase.List(c.Context(), domainInvoice.Query{
		Search: search,
		Status: status,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, items, page, limit, total)
}

func (h *InvoiceHandler) MarkAsPaid(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid invoice ID")
	}

	var req dto.MarkPaidRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return response.BadRequest(c, "Invalid request body")
		}
	}

	method := req.PaymentMethod
	if method == "" && req.Method != "" {
		method = req.Method
	}

	result, err := h.usecase.MarkAsPaid(c.Context(), usecaseInvoice.MarkPaidInput{
		InvoiceID:     uint(id),
		Amount:        req.Amount,
		PaymentMethod: method,
	})
	if err != nil {
		return err
	}

	return response.OK(c, result, "Invoice payment processed successfully")
}

func (h *InvoiceHandler) ExportPDF(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid invoice ID")
	}

	var inv domainInvoice.Invoice
	if err := h.db.WithContext(c.Context()).First(&inv, id).Error; err != nil {
		return response.NotFound(c, "Invoice not found")
	}

	// Company settings
	companyName := "Chawy ERP"
	companyAddress := "–"
	companyTaxID := "–"
	companyPhone := "–"
	companyEmail := "–"
	companyLogo := ""
	vatRate := 7.0

	if h.settingsUsecase != nil {
		if stg, err := h.settingsUsecase.GetSettings(c.Context()); err == nil && stg != nil {
			if stg.Company.Name != "" {
				companyName = stg.Company.Name
			}
			if stg.Company.Address != "" {
				companyAddress = stg.Company.Address
			}
			if stg.Company.TaxID != "" {
				companyTaxID = stg.Company.TaxID
			}
			if stg.Company.Phone != "" {
				companyPhone = stg.Company.Phone
			}
			if stg.Company.Email != "" {
				companyEmail = stg.Company.Email
			}
			if stg.Company.LogoURL != "" {
				companyLogo = stg.Company.LogoURL
			}
			if stg.Company.VatRate > 0 {
				vatRate = stg.Company.VatRate
			}
		}
	}

	var customerAddress string
	var customerTaxID string
	var pdfLines []pdf.InvoicePDFLine

	if inv.OrderID != nil && *inv.OrderID > 0 {
		var order domainOrder.Order
		if err := h.db.WithContext(c.Context()).Preload("Items").First(&order, *inv.OrderID).Error; err == nil {
			for i, it := range order.Items {
				pdfLines = append(pdfLines, pdf.InvoicePDFLine{
					Index:     i + 1,
					Name:      it.Name,
					SKU:       it.SKU,
					Quantity:  it.Quantity,
					UnitPrice: it.Price,
					LineTotal: it.Subtotal,
				})
			}
			if order.CustomerID > 0 {
				var cust domainCustomer.Customer
				if err := h.db.WithContext(c.Context()).First(&cust, order.CustomerID).Error; err == nil {
					customerAddress = cust.Address
					customerTaxID = cust.TaxID
				}
			}
		}
	}

	// Fallback if no order items found
	if len(pdfLines) == 0 {
		pdfLines = append(pdfLines, pdf.InvoicePDFLine{
			Index:     1,
			Name:      "บริการ / รายการตามใบแจ้งหนี้ " + inv.InvoiceNo,
			Quantity:  1,
			UnitPrice: inv.Amount,
			LineTotal: inv.Amount,
		})
	}

	issueDate := inv.CreatedAt.Format("02/01/2006")
	dueDate := ""
	if inv.DueDate != nil {
		dueDate = inv.DueDate.Format("02/01/2006")
	}

	pdfData := pdf.InvoicePDFData{
		InvoiceNo:       inv.InvoiceNo,
		IssueDate:       issueDate,
		DueDate:         dueDate,
		OrderNo:         inv.OrderNo,
		CompanyName:     companyName,
		CompanyAddress:  companyAddress,
		CompanyTaxID:    companyTaxID,
		CompanyPhone:    companyPhone,
		CompanyEmail:    companyEmail,
		CompanyLogo:     companyLogo,
		CustomerName:    inv.CustomerName,
		CustomerAddress: customerAddress,
		CustomerTaxID:   customerTaxID,
		Lines:           pdfLines,
		TotalAmount:     inv.Amount,
		VatRate:         vatRate,
		Status:          string(inv.Status),
	}

	pdfBytes, err := pdf.GenerateInvoicePDF(pdfData)
	if err != nil {
		return response.InternalServerError(c, "Failed to generate PDF: "+err.Error())
	}

	filename := fmt.Sprintf("Invoice-%s.pdf", inv.InvoiceNo)
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	return c.Send(pdfBytes)
}
