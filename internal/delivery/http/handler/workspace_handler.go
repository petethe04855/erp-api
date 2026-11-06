package handler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainQuotation "chawy-erp-api/internal/domain/quotation"
	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseBundle "chawy-erp-api/internal/usecase/bundle"
	usecaseCatalog "chawy-erp-api/internal/usecase/catalog"
	usecaseCustomer "chawy-erp-api/internal/usecase/customer"
	usecaseInvoice "chawy-erp-api/internal/usecase/invoice"
	usecaseOrder "chawy-erp-api/internal/usecase/order"
	usecasePurchasing "chawy-erp-api/internal/usecase/purchasing"
	usecaseQuotation "chawy-erp-api/internal/usecase/quotation"
	usecaseSeq "chawy-erp-api/internal/usecase/sequence"
	usecaseSettings "chawy-erp-api/internal/usecase/settings"
	usecaseStock "chawy-erp-api/internal/usecase/stock"
	appErrors "chawy-erp-api/pkg/errors"
	"chawy-erp-api/pkg/pdf"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type WorkspaceHandler struct {
	catalogUsecase    usecaseCatalog.Usecase
	orderUsecase      usecaseOrder.Usecase
	quotationUsecase  usecaseQuotation.Usecase
	stockUsecase      usecaseStock.Usecase
	skuRepo           domainSKU.Repository
	seqUsecase        usecaseSeq.Usecase
	bundleUsecase     usecaseBundle.Usecase
	purchasingUsecase usecasePurchasing.Usecase
	invoiceUsecase    usecaseInvoice.Usecase
	customerUsecase   usecaseCustomer.Usecase
	settingsUsecase   usecaseSettings.Usecase
}

func NewWorkspaceHandler(
	catalogUsecase usecaseCatalog.Usecase,
	orderUsecase usecaseOrder.Usecase,
	quotationUsecase usecaseQuotation.Usecase,
	stockUsecase usecaseStock.Usecase,
	skuRepo domainSKU.Repository,
	bundleUsecase usecaseBundle.Usecase,
	purchasingUsecase usecasePurchasing.Usecase,
	invoiceUsecase usecaseInvoice.Usecase,
	customerUsecase usecaseCustomer.Usecase,
	settingsUsecase usecaseSettings.Usecase,
	seqUsecase ...usecaseSeq.Usecase,
) *WorkspaceHandler {
	var su usecaseSeq.Usecase
	if len(seqUsecase) > 0 {
		su = seqUsecase[0]
	}
	return &WorkspaceHandler{
		catalogUsecase:    catalogUsecase,
		orderUsecase:      orderUsecase,
		quotationUsecase:  quotationUsecase,
		stockUsecase:      stockUsecase,
		skuRepo:           skuRepo,
		seqUsecase:        su,
		bundleUsecase:     bundleUsecase,
		purchasingUsecase: purchasingUsecase,
		invoiceUsecase:    invoiceUsecase,
		customerUsecase:   customerUsecase,
		settingsUsecase:   settingsUsecase,
	}
}

// OrderRecord matching erp-web-v2 features/erp/types/records.ts
type OrderRecord struct {
	ID       uint    `json:"id"`
	Code     string  `json:"code"`
	Customer string  `json:"customer"`
	Date     string  `json:"date"`
	Amount   float64 `json:"amount"`
	Status   string  `json:"status"`
	Channel  string  `json:"channel"`
	Items    int     `json:"items"`
	InvRef   string  `json:"invRef"`
}

func (h *WorkspaceHandler) GetOrders(c *fiber.Ctx) error {
	var bodyReq struct {
		Page          int    `json:"page"`
		Limit         int    `json:"limit"`
		Search        string `json:"search"`
		Status        string `json:"status"`
		Channel       string `json:"channel"`
		PaymentStatus string `json:"paymentStatus"`
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
	if limit < 1 || limit > 100 {
		limit = 50
	}

	search := strings.TrimSpace(bodyReq.Search)
	if search == "" {
		search = strings.TrimSpace(c.Query("search", ""))
	}

	status := strings.TrimSpace(bodyReq.Status)
	if status == "" {
		status = strings.TrimSpace(c.Query("status", ""))
	}

	channel := strings.TrimSpace(bodyReq.Channel)
	if channel == "" {
		channel = strings.TrimSpace(c.Query("channel", ""))
	}

	paymentStatus := strings.TrimSpace(bodyReq.PaymentStatus)
	if paymentStatus == "" {
		paymentStatus = strings.TrimSpace(c.Query("paymentStatus", ""))
	}

	items, total, err := h.orderUsecase.ListSales(c.Context(), domainOrder.SalesQuery{
		Search:        search,
		Status:        status,
		Channel:       channel,
		PaymentStatus: paymentStatus,
		Page:          page,
		Limit:         limit,
	})
	if err != nil {
		return response.InternalServerError(c, "Failed to load sales orders")
	}

	records := make([]OrderRecord, len(items))
	for i, item := range items {
		o := item.Order
		invRef := "â€”"
		if item.InvoiceStatus != "" {
			invRef = item.InvoiceStatus
		}

		records[i] = OrderRecord{
			ID:       o.ID,
			Code:     o.OrderNo,
			Customer: o.CustomerName,
			Date:     o.CreatedAt.Format("2006-01-02"),
			Amount:   o.TotalAmount,
			Status:   string(o.Status),
			Channel:  o.Channel,
			Items:    len(o.Items),
			InvRef:   invRef,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) CreateSalesOrder(c *fiber.Ctx) error {
	var req struct {
		Customer   string `json:"customer"`
		Channel    string `json:"channel"`
		IncludeVat *bool  `json:"includeVat"`
		Lines      []struct {
			SKU       string  `json:"sku"`
			Qty       int     `json:"qty"`
			UnitPrice float64 `json:"unitPrice"`
		} `json:"lines"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var items []usecaseOrder.CreateItemInput
	for _, l := range req.Lines {
		if l.Qty <= 0 {
			return response.BadRequest(c, fmt.Sprintf("Quantity for SKU %s must be greater than 0", l.SKU))
		}
		items = append(items, usecaseOrder.CreateItemInput{
			SKU:      l.SKU,
			Quantity: l.Qty,
			Price:    l.UnitPrice,
		})
	}

	channel := req.Channel
	if channel == "" {
		channel = "Manual"
	}

	includeVat := true
	if req.IncludeVat != nil {
		includeVat = *req.IncludeVat
	}

	// FULL-17: VAT policy travels with the order; totals are computed once
	// by the Usecase. The handler never overwrites amounts afterwards.
	note := "VAT_INC:true"
	if !includeVat {
		note = "VAT_INC:false"
	}

	order, err := h.orderUsecase.Create(c.Context(), usecaseOrder.CreateOrderInput{
		CustomerName: req.Customer,
		Channel:      channel,
		Note:         note,
		Items:        items,
		VATIncluded:  includeVat,
	})
	if err != nil {
		return err
	}

	rec := OrderRecord{
		ID:       order.ID,
		Code:     order.OrderNo,
		Customer: order.CustomerName,
		Date:     order.CreatedAt.Format("2006-01-02"),
		Amount:   order.TotalAmount,
		Status:   string(order.Status),
		Channel:  order.Channel,
		Items:    len(order.Items),
		InvRef:   "—",
	}
	return response.Created(c, rec, "Sales order created successfully")
}

func (h *WorkspaceHandler) GetSalesOrderByID(c *fiber.Ctx) error {
	ord, err := h.orderUsecase.GetByRef(c.Context(), c.Params("id"))
	if err != nil {
		return response.InternalServerError(c, "Failed to load sales order")
	}
	if ord == nil {
		return response.NotFound(c, "Sales order not found")
	}

	type LineItem struct {
		SKU       string  `json:"sku"`
		Name      string  `json:"name"`
		Quantity  int     `json:"quantity"`
		UnitPrice float64 `json:"unitPrice"`
		Subtotal  float64 `json:"subtotal"`
	}

	lines := make([]LineItem, len(ord.Items))
	for i, item := range ord.Items {
		lines[i] = LineItem{
			SKU:       item.SKU,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.Price,
			Subtotal:  item.Subtotal,
		}
	}

	includeVat := !strings.Contains(ord.Note, "VAT_INC:false")

	result := fiber.Map{
		"id":         ord.ID,
		"code":       ord.OrderNo,
		"customer":   ord.CustomerName,
		"channel":    ord.Channel,
		"date":       ord.CreatedAt.Format("2006-01-02 15:04"),
		"amount":     ord.TotalAmount,
		"status":     string(ord.Status),
		"note":       ord.Note,
		"includeVat": includeVat,
		"lines":      lines,
		"itemsCount": len(ord.Items),
	}

	return response.OK(c, result)
}

func (h *WorkspaceHandler) ExportSalesOrderPDF(c *fiber.Ctx) error {
	ord, err := h.orderUsecase.GetByRef(c.Context(), c.Params("id"))
	if err != nil {
		return response.InternalServerError(c, "Failed to load sales order")
	}
	if ord == nil {
		return response.NotFound(c, "Sales order not found")
	}

	company := h.loadCompanyInfo(c.Context())

	var customerAddress string
	var customerTaxID string
	if ord.CustomerID > 0 {
		if cust, err := h.customerUsecase.GetByID(c.Context(), ord.CustomerID); err == nil && cust != nil {
			customerAddress = cust.Address
			customerTaxID = cust.TaxID
		}
	}

	pdfLines := make([]pdf.SalesOrderPDFLine, 0, len(ord.Items))
	for i, item := range ord.Items {
		pdfLines = append(pdfLines, pdf.SalesOrderPDFLine{
			Index:     i + 1,
			Name:      item.Name,
			SKU:       item.SKU,
			Quantity:  item.Quantity,
			UnitPrice: item.Price,
			LineTotal: item.Subtotal,
		})
	}

	includeVat := !strings.Contains(ord.Note, "VAT_INC:false")
	noteForPdf := strings.TrimSpace(strings.ReplaceAll(ord.Note, "VAT_INC:false", ""))

	pdfData := pdf.SalesOrderPDFData{
		Code:            ord.OrderNo,
		Date:            ord.CreatedAt.Format("02/01/2006"),
		Channel:         ord.Channel,
		CompanyName:     company.Name,
		CompanyAddress:  company.Address,
		CompanyTaxID:    company.TaxID,
		CompanyPhone:    company.Phone,
		CompanyEmail:    company.Email,
		CompanyLogo:     company.Logo,
		CustomerName:    ord.CustomerName,
		CustomerAddress: customerAddress,
		CustomerTaxID:   customerTaxID,
		Lines:           pdfLines,
		TotalAmount:     ord.TotalAmount,
		VatRate:         7.0,
		IncludeVat:      includeVat,
		Note:            noteForPdf,
		Status:          string(ord.Status),
	}

	pdfBytes, err := pdf.GenerateSalesOrderPDF(pdfData)
	if err != nil {
		return response.InternalServerError(c, "Failed to generate sales order PDF: "+err.Error())
	}

	filename := fmt.Sprintf("SalesOrder-%s.pdf", ord.OrderNo)
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	return c.Send(pdfBytes)
}

func (h *WorkspaceHandler) UpdateSalesOrderStatus(c *fiber.Ctx) error {
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	ord, already, err := h.orderUsecase.UpdateStatusByRef(c.Context(), c.Params("id"), req.Status, req.Note)
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		return response.InternalServerError(c, "Failed to update order status")
	}

	if already {
		return response.OK(c, ord, fmt.Sprintf("Sales order is already %s", ord.Status))
	}

	switch ord.Status {
	case domainOrder.StatusCancelled:
		return response.OK(c, ord, "Sales order cancelled successfully")
	case domainOrder.StatusShipped:
		return response.OK(c, ord, "Sales order completed and stock deducted successfully")
	}

	return response.OK(c, ord, "Sales order status updated")
}

// InvoiceRecord matching erp-web-v2 features/erp/types/records.ts
type InvoiceRecord struct {
	ID        uint    `json:"id"`
	Code      string  `json:"code"`
	SORef     string  `json:"soRef"`
	Customer  string  `json:"customer"`
	IssueDate string  `json:"issueDate"`
	DueDate   string  `json:"dueDate"`
	Subtotal  float64 `json:"subtotal"`
	VATAmount float64 `json:"vatAmount"`
	Amount    float64 `json:"amount"`
	Paid      float64 `json:"paid"`
	Balance   float64 `json:"balance"`
	IsOverdue bool    `json:"isOverdue"`
	Status    string  `json:"status"`
}

func (h *WorkspaceHandler) GetInvoices(c *fiber.Ctx) error {
	var bodyReq struct {
		Page   int    `json:"page"`
		Limit  int    `json:"limit"`
		Search string `json:"search"`
		Status string `json:"status"`
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
	if limit < 1 || limit > 100 {
		limit = 50
	}

	search := strings.TrimSpace(bodyReq.Search)
	if search == "" {
		search = strings.TrimSpace(c.Query("search", ""))
	}

	statusQuery := strings.TrimSpace(bodyReq.Status)
	if statusQuery == "" {
		statusQuery = strings.TrimSpace(c.Query("status", ""))
	}

	query := domainInvoice.Query{
		Search: search,
		Page:   page,
		Limit:  limit,
	}
	if strings.EqualFold(statusQuery, "overdue") {
		// Overdue: balance > 0 and due_date < today and not PAID/CANCELLED
		query.OverdueOnly = true
	} else if statusQuery != "" && !strings.EqualFold(statusQuery, "all") {
		query.Status = strings.ToUpper(statusQuery)
	}

	invoices, total, err := h.invoiceUsecase.List(c.Context(), query)
	if err != nil {
		return response.InternalServerError(c, "Failed to load invoices")
	}

	today := time.Now().Truncate(24 * time.Hour)

	records := make([]InvoiceRecord, len(invoices))
	for i, inv := range invoices {
		paid := inv.PaidAmount
		if inv.Status == domainInvoice.StatusPaid && paid <= 0 {
			paid = inv.Amount
		}
		balance := inv.Amount - paid
		if balance < 0 {
			balance = 0
		}
		dueDateStr := ""
		isOverdue := false
		if inv.DueDate != nil {
			dueDateStr = inv.DueDate.Format("2006-01-02")
			if balance > 0 && inv.Status != domainInvoice.StatusPaid && inv.Status != domainInvoice.StatusCancelled && inv.DueDate.Before(today) {
				isOverdue = true
			}
		}
		soRef := inv.OrderNo
		if soRef == "" && inv.OrderID != nil {
			soRef = fmt.Sprintf("ORD-%04d", *inv.OrderID)
		}

		records[i] = InvoiceRecord{
			ID:        inv.ID,
			Code:      inv.InvoiceNo,
			SORef:     soRef,
			Customer:  inv.CustomerName,
			IssueDate: inv.CreatedAt.Format("2006-01-02"),
			DueDate:   dueDateStr,
			Subtotal:  inv.Amount / 1.07,
			VATAmount: inv.Amount - (inv.Amount / 1.07),
			Amount:    inv.Amount,
			Paid:      paid,
			Balance:   balance,
			IsOverdue: isOverdue,
			Status:    string(inv.Status),
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) CreateInvoiceFromSO(c *fiber.Ctx) error {
	inv, existed, err := h.invoiceUsecase.CreateFromSalesOrderRef(c.Context(), c.Params("soRef"))
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		if errors.Is(err, appErrors.ErrNotFound) {
			return response.NotFound(c, "Sales order not found")
		}
		return response.InternalServerError(c, "Failed to create invoice")
	}

	soRef := inv.OrderNo
	if soRef == "" && inv.OrderID != nil {
		soRef = fmt.Sprintf("ORD-%04d", *inv.OrderID)
	}
	dueDateStr := ""
	if inv.DueDate != nil {
		dueDateStr = inv.DueDate.Format("2006-01-02")
	}
	paid := inv.PaidAmount
	if inv.Status == domainInvoice.StatusPaid && paid <= 0 {
		paid = inv.Amount
	}
	balance := inv.Amount - paid
	if balance < 0 {
		balance = 0
	}

	rec := InvoiceRecord{
		ID:        inv.ID,
		Code:      inv.InvoiceNo,
		SORef:     soRef,
		Customer:  inv.CustomerName,
		IssueDate: inv.CreatedAt.Format("2006-01-02"),
		DueDate:   dueDateStr,
		Subtotal:  inv.Amount / 1.07,
		VATAmount: inv.Amount - (inv.Amount / 1.07),
		Amount:    inv.Amount,
		Paid:      paid,
		Balance:   balance,
		Status:    string(inv.Status),
	}

	if existed {
		return response.OK(c, rec, "Existing invoice retrieved")
	}
	return response.Created(c, rec, "Invoice created successfully")
}

func (h *WorkspaceHandler) GetInvoiceByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid invoice ID")
	}

	inv, order, cust, err := h.invoiceUsecase.GetInvoiceDetailForPDF(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return response.NotFound(c, "Invoice not found")
		}
		return response.InternalServerError(c, "Failed to load invoice")
	}
	if inv == nil {
		return response.NotFound(c, "Invoice not found")
	}

	var lines []fiber.Map
	var customerAddress string
	var customerLogo string

	if order != nil {
		for _, it := range order.Items {
			lines = append(lines, fiber.Map{
				"sku":       it.SKU,
				"name":      it.Name,
				"qty":       it.Quantity,
				"price":     it.Price,
				"unitPrice": it.Price,
				"subtotal":  it.Subtotal,
			})
		}
	}
	if cust != nil {
		customerAddress = cust.Address
		customerLogo = cust.Logo
	}

	dueDateStr := ""
	if inv.DueDate != nil {
		dueDateStr = inv.DueDate.Format("2006-01-02")
	}

	// FULL-19: report the real paid amount for partial payments, consistent
	// with the invoice list endpoint.
	paidAmount := inv.PaidAmount
	if inv.Status == domainInvoice.StatusPaid && paidAmount <= 0 {
		paidAmount = inv.Amount
	}
	balance := inv.Amount - paidAmount
	if balance < 0 {
		balance = 0
	}
	isOverdue := false
	today := time.Now().Truncate(24 * time.Hour)
	if inv.DueDate != nil && balance > 0 && inv.Status != domainInvoice.StatusPaid && inv.Status != domainInvoice.StatusCancelled && inv.DueDate.Before(today) {
		isOverdue = true
	}

	return response.OK(c, fiber.Map{
		"id":              inv.ID,
		"code":            inv.InvoiceNo,
		"invoiceNo":       inv.InvoiceNo,
		"soRef":           inv.OrderNo,
		"orderNo":         inv.OrderNo,
		"orderId":         inv.OrderID,
		"customer":        inv.CustomerName,
		"customerName":    inv.CustomerName,
		"customerAddress": customerAddress,
		"customerLogo":    customerLogo,
		"amount":          inv.Amount,
		"totalAmount":     inv.Amount,
		"paid":            paidAmount,
		"balance":         balance,
		"isOverdue":       isOverdue,
		"status":          string(inv.Status),
		"paymentMethod":   inv.PaymentMethod,
		"issueDate":       inv.CreatedAt.Format("2006-01-02"),
		"date":            inv.CreatedAt.Format("2006-01-02 15:04"),
		"dueDate":         dueDateStr,
		"lines":           lines,
		"itemsCount":      len(lines),
	})
}

// PurchaseRecord matching erp-web-v2 features/erp/types/records.ts
type PurchaseRecord struct {
	ID        uint    `json:"id"`
	Code      string  `json:"code"`
	Supplier  string  `json:"supplier"`
	Date      string  `json:"date"`
	ETADate   string  `json:"etaDate"`
	TotalCost float64 `json:"totalCost"`
	Status    string  `json:"status"`
}

func (h *WorkspaceHandler) GetPurchaseOrders(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	search := strings.TrimSpace(c.Query("search", ""))
	statusQuery := strings.TrimSpace(c.Query("status", ""))

	if c.Method() == fiber.MethodPost {
		var body struct {
			Page   int    `json:"page"`
			Limit  int    `json:"limit"`
			Search string `json:"search"`
			Status string `json:"status"`
		}
		if err := c.BodyParser(&body); err == nil {
			if body.Page > 0 {
				page = body.Page
			}
			if body.Limit > 0 {
				limit = body.Limit
			}
			if strings.TrimSpace(body.Search) != "" {
				search = strings.TrimSpace(body.Search)
			}
			if strings.TrimSpace(body.Status) != "" {
				statusQuery = strings.TrimSpace(body.Status)
			}
		}
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	query := domainPurchasing.POQuery{Search: search, Page: page, Limit: limit}
	if statusQuery != "" && !strings.EqualFold(statusQuery, "all") {
		// The repository matches the stored status exactly; the screen sends
		// lower-case values, so normalise the casing here.
		query.Status = strings.ToUpper(statusQuery)
	}

	pos, total, err := h.purchasingUsecase.ListPOs(c.Context(), query)
	if err != nil {
		return response.InternalServerError(c, "Failed to load purchase orders")
	}

	records := make([]PurchaseRecord, len(pos))
	for i, po := range pos {
		supName := "Unknown Supplier"
		if po.SupplierName != "" {
			supName = po.SupplierName
		}
		etaDate := po.ETADate
		if etaDate == "" {
			etaDate = po.CreatedAt.AddDate(0, 0, 7).Format("2006-01-02")
		}
		records[i] = PurchaseRecord{
			ID:        po.ID,
			Code:      po.PONo,
			Supplier:  supName,
			Date:      po.CreatedAt.Format("2006-01-02"),
			ETADate:   etaDate,
			TotalCost: po.TotalAmount,
			Status:    string(po.Status),
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) GetPurchaseOrderByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid purchase order ID")
	}

	po, err := h.purchasingUsecase.GetPOByID(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return response.NotFound(c, "Purchase order not found")
		}
		return response.InternalServerError(c, "Failed to load purchase order")
	}

	items := make([]fiber.Map, len(po.Items))
	for i, item := range po.Items {
		items[i] = fiber.Map{
			"id":          item.ID,
			"sku":         item.SKU,
			"name":        item.Name,
			"qty":         item.Quantity,
			"receivedQty": item.ReceivedQty,
			"price":       item.UnitCost,
			"subtotal":    item.Subtotal,
		}
	}

	etaDate := po.ETADate
	if etaDate == "" {
		etaDate = po.CreatedAt.AddDate(0, 0, 7).Format("2006-01-02")
	}

	return response.OK(c, fiber.Map{
		"id":         po.ID,
		"code":       po.PONo,
		"supplier":   po.SupplierName,
		"status":     string(po.Status),
		"amount":     po.TotalAmount,
		"totalCost":  po.TotalAmount,
		"date":       po.CreatedAt.Format("2006-01-02 15:04"),
		"etaDate":    etaDate,
		"note":       po.Note,
		"lines":      items,
		"itemsCount": len(po.Items),
	})
}

func (h *WorkspaceHandler) CreatePurchaseOrder(c *fiber.Ctx) error {
	var req struct {
		Supplier             string `json:"supplier"`
		SupplierID           uint   `json:"supplierId"`
		Note                 string `json:"note"`
		ETADate              string `json:"etaDate"`
		ExpectedDeliveryDays int    `json:"expectedDeliveryDays"`
		Items                []struct {
			SKU      string  `json:"sku"`
			Name     string  `json:"name"`
			Quantity int     `json:"quantity"`
			Qty      int     `json:"qty"`
			UnitCost float64 `json:"unitCost"`
		} `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	items := make([]usecasePurchasing.CreatePurchaseOrderItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = usecasePurchasing.CreatePurchaseOrderItem{
			SKU:      it.SKU,
			Name:     it.Name,
			Quantity: it.Quantity,
			Qty:      it.Qty,
			UnitCost: it.UnitCost,
		}
	}

	po, err := h.purchasingUsecase.CreatePurchaseOrder(c.Context(), usecasePurchasing.CreatePurchaseOrderInput{
		Supplier:             req.Supplier,
		SupplierID:           req.SupplierID,
		Note:                 req.Note,
		ETADate:              req.ETADate,
		ExpectedDeliveryDays: req.ExpectedDeliveryDays,
		Items:                items,
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		return response.InternalServerError(c, "Failed to create purchase order: "+err.Error())
	}

	etaDate := po.ETADate
	if etaDate == "" {
		etaDate = po.CreatedAt.AddDate(0, 0, 7).Format("2006-01-02")
	}

	return response.Created(c, PurchaseRecord{
		ID:        po.ID,
		Code:      po.PONo,
		Supplier:  po.SupplierName,
		Date:      po.CreatedAt.Format("2006-01-02"),
		ETADate:   etaDate,
		TotalCost: po.TotalAmount,
		Status:    string(po.Status),
	}, "Purchase order created successfully")
}

func (h *WorkspaceHandler) UpdatePurchaseOrderStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid purchase order ID")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	po, err := h.purchasingUsecase.UpdatePOStatus(c.Context(), uint(id), req.Status)
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		return response.InternalServerError(c, "Failed to update purchase order status")
	}

	return response.OK(c, po, "Purchase order status updated")
}

// QuotationRecord matching erp-web-v2 features/erp/types/records.ts
type QuotationRecord struct {
	ID         uint    `json:"id"`
	Code       string  `json:"code"`
	Customer   string  `json:"customer"`
	Date       string  `json:"date"`
	ValidUntil string  `json:"validUntil"`
	LeadSource string  `json:"leadSource"`
	Amount     float64 `json:"amount"`
	IsExpired  bool    `json:"isExpired"`
	Status     string  `json:"status"`
}

func (h *WorkspaceHandler) GetQuotations(c *fiber.Ctx) error {
	var bodyReq struct {
		Page   int    `json:"page"`
		Limit  int    `json:"limit"`
		Search string `json:"search"`
		Status string `json:"status"`
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
	if limit < 1 || limit > 100 {
		limit = 50
	}

	search := strings.TrimSpace(bodyReq.Search)
	if search == "" {
		search = strings.TrimSpace(c.Query("search", ""))
	}

	statusQuery := strings.TrimSpace(bodyReq.Status)
	if statusQuery == "" {
		statusQuery = strings.TrimSpace(c.Query("status", ""))
	}

	query := domainQuotation.Query{Search: search, Page: page, Limit: limit}
	if strings.EqualFold(statusQuery, "expired") {
		// Expired: valid_until < today AND status IN ('Pending', 'Draft', 'Sent', 'Approved')
		query.ExpiredOnly = true
	} else if statusQuery != "" && !strings.EqualFold(statusQuery, "all") {
		query.Status = statusQuery
	}

	quotations, total, err := h.quotationUsecase.ListFiltered(c.Context(), query)
	if err != nil {
		return response.InternalServerError(c, "Failed to load quotations")
	}

	today := time.Now().Format("2006-01-02")

	records := make([]QuotationRecord, len(quotations))
	for i, q := range quotations {
		isExpired := false
		if (q.Status == domainQuotation.StatusPending || q.Status == domainQuotation.StatusDraft || q.Status == domainQuotation.StatusSent || q.Status == domainQuotation.StatusApproved) && q.ValidUntil != "" && q.ValidUntil < today {
			isExpired = true
		}
		records[i] = QuotationRecord{
			ID:         q.ID,
			Code:       q.Code,
			Customer:   q.CustomerName,
			Date:       q.Date,
			ValidUntil: q.ValidUntil,
			LeadSource: q.LeadSource,
			Amount:     q.TotalAmount,
			IsExpired:  isExpired,
			Status:     string(q.Status),
		}
	}
	return response.List(c, records, page, limit, total)
}

// ResolveSKUs resolves a batch of SKU codes in one round trip so clients
// building quotations/POs don't issue one GET per line (N+1).
func (h *WorkspaceHandler) ResolveSKUs(c *fiber.Ctx) error {
	var req struct {
		SKUs []string `json:"skus"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if len(req.SKUs) == 0 {
		return response.BadRequest(c, "skus cannot be empty")
	}
	if len(req.SKUs) > 200 {
		return response.BadRequest(c, "skus exceeds the maximum of 200 codes per request")
	}

	normalized := make([]string, 0, len(req.SKUs))
	seen := make(map[string]bool, len(req.SKUs))
	for _, raw := range req.SKUs {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code != "" && !seen[code] {
			seen[code] = true
			normalized = append(normalized, code)
		}
	}
	if len(normalized) == 0 {
		return response.BadRequest(c, "skus cannot be empty")
	}

	// Resolution (SKU master + active Inventory Formula fallback) lives in the
	// catalog usecase; the handler keeps request validation and normalization.
	records, err := h.catalogUsecase.Resolve(c.Context(), normalized)
	if err != nil {
		return respondCatalogError(c, err)
	}

	return response.OK(c, records)
}

func (h *WorkspaceHandler) CreateQuotation(c *fiber.Ctx) error {
	var req struct {
		Customer   string `json:"customer"`
		Date       string `json:"date"`
		ValidUntil string `json:"validUntil"`
		Status     string `json:"status"`
		LeadSource string `json:"leadSource"`
		Note       string `json:"note"`
		Lines      []struct {
			ProductID uint    `json:"productId"`
			SKU       string  `json:"sku"`
			Name      string  `json:"name"`
			Price     float64 `json:"price"`
			Qty       int     `json:"qty"`
		} `json:"lines"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	input := usecaseQuotation.CreateInput{
		Customer:   req.Customer,
		Date:       req.Date,
		ValidUntil: req.ValidUntil,
		Status:     req.Status,
		LeadSource: req.LeadSource,
		Note:       req.Note,
	}
	input.Lines = make([]usecaseQuotation.LineInput, len(req.Lines))
	for i, l := range req.Lines {
		input.Lines[i] = usecaseQuotation.LineInput{
			ProductID: l.ProductID,
			SKU:       l.SKU,
			Name:      l.Name,
			Price:     l.Price,
			Qty:       l.Qty,
		}
	}

	q, err := h.quotationUsecase.Create(c.Context(), input)
	if err != nil {
		return err
	}

	rec := QuotationRecord{
		ID:         q.ID,
		Code:       q.Code,
		Customer:   q.CustomerName,
		Date:       q.Date,
		ValidUntil: q.ValidUntil,
		LeadSource: q.LeadSource,
		Amount:     q.TotalAmount,
		Status:     string(q.Status),
	}
	return response.Created(c, rec, "Quotation created successfully")
}

func (h *WorkspaceHandler) GetQuotationByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	q, err := h.quotationUsecase.GetByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	lines := make([]fiber.Map, len(q.Lines))
	for i, l := range q.Lines {
		lines[i] = fiber.Map{
			"id":        l.ID,
			"productId": l.ProductID,
			"sku":       l.SKU,
			"name":      l.Name,
			"price":     l.Price,
			"qty":       l.Quantity,
			"subtotal":  l.Subtotal,
		}
	}

	today := time.Now().Format("2006-01-02")
	isExpired := false
	if (q.Status == domainQuotation.StatusPending || q.Status == domainQuotation.StatusDraft || q.Status == domainQuotation.StatusSent || q.Status == domainQuotation.StatusApproved) && q.ValidUntil != "" && q.ValidUntil < today {
		isExpired = true
	}

	return response.OK(c, fiber.Map{
		"id":         q.ID,
		"code":       q.Code,
		"customer":   q.CustomerName,
		"date":       q.Date,
		"validUntil": q.ValidUntil,
		"amount":     q.TotalAmount,
		"status":     string(q.Status),
		"isExpired":  isExpired,
		"leadSource": q.LeadSource,
		"note":       q.Note,
		"lines":      lines,
		"itemsCount": len(q.Lines),
	})
}

func (h *WorkspaceHandler) ExportQuotationPDF(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	q, err := h.quotationUsecase.GetByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	company := h.loadCompanyInfo(c.Context())

	var customerAddress string
	var customerTaxID string
	if q.CustomerID > 0 {
		if cust, err := h.customerUsecase.GetByID(c.Context(), q.CustomerID); err == nil && cust != nil {
			customerAddress = cust.Address
			customerTaxID = cust.TaxID
		}
	}

	var pdfLines []pdf.QuotationPDFLine
	for i, l := range q.Lines {
		pdfLines = append(pdfLines, pdf.QuotationPDFLine{
			Index:     i + 1,
			Name:      l.Name,
			SKU:       l.SKU,
			Quantity:  l.Quantity,
			UnitPrice: l.Price,
			LineTotal: l.Subtotal,
		})
	}

	pdfData := pdf.QuotationPDFData{
		Code:            q.Code,
		Date:            q.Date,
		ValidUntil:      q.ValidUntil,
		LeadSource:      q.LeadSource,
		CompanyName:     company.Name,
		CompanyAddress:  company.Address,
		CompanyTaxID:    company.TaxID,
		CompanyPhone:    company.Phone,
		CompanyEmail:    company.Email,
		CompanyLogo:     company.Logo,
		CustomerName:    q.CustomerName,
		CustomerAddress: customerAddress,
		CustomerTaxID:   customerTaxID,
		Lines:           pdfLines,
		TotalAmount:     q.TotalAmount,
		VatRate:         7.0,
		Note:            q.Note,
		Status:          string(q.Status),
	}

	pdfBytes, err := pdf.GenerateQuotationPDF(pdfData)
	if err != nil {
		return response.InternalServerError(c, "Failed to generate quotation PDF: "+err.Error())
	}

	filename := fmt.Sprintf("Quotation-%s.pdf", q.Code)
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filename))
	return c.Send(pdfBytes)
}

func (h *WorkspaceHandler) UpdateQuotationStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	q, err := h.quotationUsecase.UpdateStatus(c.Context(), uint(id), domainQuotation.Status(req.Status))
	if err != nil {
		return err
	}

	return response.OK(c, q, "Quotation status updated")
}

func (h *WorkspaceHandler) ConvertQuotationToSO(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	result, err := h.quotationUsecase.ConvertToSalesOrder(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, fiber.Map{
		"quotationId": result.QuotationID,
		"orderId":     result.OrderID,
		"orderNo":     result.OrderNo,
	}, "Quotation converted to Sales Order successfully")
}

// ReceiptRecord matching erp-web-v2 features/erp/types/records.ts
type ReceiptRecord struct {
	ID          uint   `json:"id"`
	Code        string `json:"code"`
	PORef       string `json:"poRef"`
	ReceiveDate string `json:"receiveDate"`
	Note        string `json:"note"`
}

func (h *WorkspaceHandler) GetGoodsReceives(c *fiber.Ctx) error {
	var bodyReq struct {
		Page   int    `json:"page"`
		Limit  int    `json:"limit"`
		Search string `json:"search"`
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
	if limit < 1 || limit > 100 {
		limit = 50
	}

	search := strings.TrimSpace(bodyReq.Search)
	if search == "" {
		search = strings.TrimSpace(c.Query("search", ""))
	}

	grs, total, err := h.purchasingUsecase.ListGoodsReceives(c.Context(), domainPurchasing.GoodsReceiveQuery{
		Search: search,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		return response.InternalServerError(c, "Failed to load goods receives")
	}

	records := make([]ReceiptRecord, len(grs))
	for i, gr := range grs {
		receiveDate := gr.ReceiveDate
		if receiveDate == "" {
			receiveDate = gr.CreatedAt.Format("2006-01-02")
		}
		records[i] = ReceiptRecord{
			ID:          gr.ID,
			Code:        gr.Code,
			PORef:       gr.PORef,
			ReceiveDate: receiveDate,
			Note:        gr.Note,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) GetGoodsReceiveByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid goods receive ID")
	}

	gr, err := h.purchasingUsecase.GetGoodsReceive(c.Context(), uint(id))
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		return response.InternalServerError(c, "Failed to load goods receive")
	}

	items := make([]fiber.Map, len(gr.Items))
	for i, item := range gr.Items {
		items[i] = fiber.Map{
			"id":          item.ID,
			"sku":         item.SKU,
			"name":        item.Name,
			"qty":         item.Quantity,
			"receivedQty": item.Quantity,
			"unitCost":    item.UnitCost,
			"retailPrice": item.RetailPrice,
			"supplierLot": item.SupplierLot,
			"expiryDate":  item.ExpiryDate,
			"qcStatus":    item.QCStatus,
		}
	}

	receiveDate := gr.ReceiveDate
	if receiveDate == "" {
		receiveDate = gr.CreatedAt.Format("2006-01-02")
	}

	return response.OK(c, fiber.Map{
		"id":          gr.ID,
		"code":        gr.Code,
		"poRef":       gr.PORef,
		"receiveDate": receiveDate,
		"supplier":    gr.SupplierName,
		"note":        gr.Note,
		"lines":       items,
		"itemsCount":  len(gr.Items),
	})
}

func (h *WorkspaceHandler) CreateGoodsReceive(c *fiber.Ctx) error {
	var req struct {
		PORef       string `json:"poRef"`
		ReceiveDate string `json:"receiveDate"`
		Note        string `json:"note"`
		Items       []struct {
			SKU         string   `json:"sku"`
			QtyReceived int      `json:"qtyReceived"`
			Quantity    int      `json:"quantity"`
			UnitCost    *float64 `json:"unitCost"`
			RetailPrice *float64 `json:"retailPrice"`
			ExpiryDate  string   `json:"expiryDate"`
			SupplierLot string   `json:"supplierLot"`
			QCStatus    string   `json:"qcStatus"`
		} `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	items := make([]usecasePurchasing.GoodsReceiveItemInput, len(req.Items))
	for i, it := range req.Items {
		items[i] = usecasePurchasing.GoodsReceiveItemInput{
			SKU:         it.SKU,
			QtyReceived: it.QtyReceived,
			Quantity:    it.Quantity,
			UnitCost:    it.UnitCost,
			RetailPrice: it.RetailPrice,
			ExpiryDate:  it.ExpiryDate,
			SupplierLot: it.SupplierLot,
			QCStatus:    it.QCStatus,
		}
	}

	gr, err := h.purchasingUsecase.CreateGoodsReceive(c.Context(), usecasePurchasing.CreateGoodsReceiveInput{
		PORef:       req.PORef,
		ReceiveDate: req.ReceiveDate,
		Note:        req.Note,
		Items:       items,
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		return response.BadRequest(c, "Failed to receive goods: "+err.Error())
	}

	return response.Created(c, ReceiptRecord{
		ID:          gr.ID,
		Code:        gr.Code,
		PORef:       gr.PORef,
		ReceiveDate: gr.ReceiveDate,
		Note:        gr.Note,
	}, "Goods received successfully")
}

// IssueRecord matching erp-web-v2 features/erp/types/records.ts
type IssueRecord struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	SKU      string `json:"sku"`
	SKUName  string `json:"skuName"`
	Qty      int    `json:"qty"`
	Reason   string `json:"reason"`
	Date     string `json:"date"`
	Channel  string `json:"channel"`
	OrderRef string `json:"orderRef"`
}

// formatMovementChannel formats raw reference type or channel to user-friendly "Manual", "Shopee", "TikTok"
func formatMovementChannel(channel, orderRef string) string {
	u := strings.ToUpper(strings.TrimSpace(channel))
	if strings.Contains(u, "TIKTOK") {
		return "TikTok"
	}
	if strings.Contains(u, "SHOPEE") {
		return "Shopee"
	}
	if strings.Contains(u, "MANUAL") {
		return "Manual"
	}
	if strings.Contains(u, "ORDER") {
		ref := strings.ToUpper(strings.TrimSpace(orderRef))
		if strings.HasPrefix(ref, "TT") || strings.Contains(ref, "TIKTOK") {
			return "TikTok"
		}
		if strings.HasPrefix(ref, "SP") || strings.Contains(ref, "SHOPEE") {
			return "Shopee"
		}
		return "Manual"
	}
	if channel == "" {
		return "Manual"
	}
	return channel
}

// asAppError returns the *AppError carried by err, or nil when err is not an
// application error (i.e. an unexpected/internal failure).
func asAppError(err error) *appErrors.AppError {
	var appErr *appErrors.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return nil
}

// respondAppError maps an application error to the matching HTTP response so
// callers keep the same status codes and messages they had before.
// companyInfo carries the company header fields printed on PDF documents.
type companyInfo struct {
	Name    string
	Address string
	TaxID   string
	Phone   string
	Email   string
	Logo    string
}

// loadCompanyInfo reads the company profile through the settings usecase so
// the handler never touches the database directly. Missing values fall back to
// the same placeholders the PDFs used before.
func (h *WorkspaceHandler) loadCompanyInfo(ctx context.Context) companyInfo {
	info := companyInfo{
		Name:    "Chawy ERP",
		Address: "â€“",
		TaxID:   "â€“",
		Phone:   "â€“",
		Email:   "â€“",
	}
	if h.settingsUsecase == nil {
		return info
	}
	settings, err := h.settingsUsecase.GetSettings(ctx)
	if err != nil || settings == nil {
		return info
	}
	company := settings.Company
	if company.Name != "" {
		info.Name = company.Name
	}
	if company.Address != "" {
		info.Address = company.Address
	}
	if company.TaxID != "" {
		info.TaxID = company.TaxID
	}
	if company.Phone != "" {
		info.Phone = company.Phone
	}
	if company.Email != "" {
		info.Email = company.Email
	}
	if company.LogoURL != "" {
		info.Logo = company.LogoURL
	}
	return info
}

// respondAppError maps an application error to the matching HTTP response so
// callers keep the same status codes and messages they had before.
func respondAppError(c *fiber.Ctx, appErr *appErrors.AppError) error {
	switch appErr.StatusCode {
	case fiber.StatusNotFound:
		return response.NotFound(c, appErr.Message)
	case fiber.StatusInternalServerError:
		return response.InternalServerError(c, appErr.Message)
	default:
		return response.BadRequest(c, appErr.Message)
	}
}

func (h *WorkspaceHandler) GetGoodsIssues(c *fiber.Ctx) error {
	var bodyReq struct {
		Page   int    `json:"page"`
		Limit  int    `json:"limit"`
		Search string `json:"search"`
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
	if limit < 1 || limit > 100 {
		limit = 50
	}

	search := strings.TrimSpace(bodyReq.Search)
	if search == "" {
		search = strings.TrimSpace(c.Query("search", ""))
	}

	// Goods issue history is backed by OUT stock movements.
	movements, total, err := h.purchasingUsecase.ListGoodsIssues(c.Context(), domainPurchasing.GoodsIssueQuery{
		Search: search,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		return response.InternalServerError(c, "Failed to load goods issues")
	}

	records := make([]IssueRecord, len(movements))
	for i, m := range movements {
		records[i] = IssueRecord{
			ID:       m.ID,
			Code:     fmt.Sprintf("GI-%s-%04d", m.CreatedAt.Format("2006/01/02"), m.ID),
			SKU:      m.SKUCode,
			SKUName:  m.SKUCode,
			Qty:      m.Quantity,
			Reason:   m.Note,
			Date:     m.CreatedAt.Format("2006-01-02"),
			Channel:  formatMovementChannel(m.ReferenceType, m.ReferenceID),
			OrderRef: m.ReferenceID,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) GetGoodsIssueByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid goods issue ID")
	}

	m, err := h.purchasingUsecase.GetGoodsIssue(c.Context(), uint(id))
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		return response.InternalServerError(c, "Failed to load goods issue")
	}

	return response.OK(c, fiber.Map{
		"id":       m.ID,
		"code":     fmt.Sprintf("GI-%04d", m.ID),
		"sku":      m.SKUCode,
		"skuName":  m.SKUCode,
		"qty":      m.Quantity,
		"reason":   m.Note,
		"date":     m.CreatedAt.Format("2006-01-02 15:04"),
		"channel":  formatMovementChannel(m.ReferenceType, m.ReferenceID),
		"orderRef": m.ReferenceID,
	})
}

func (h *WorkspaceHandler) CreateGoodsIssue(c *fiber.Ctx) error {
	var req struct {
		SKU      string `json:"sku"`
		Qty      int    `json:"qty"`
		Reason   string `json:"reason"`
		Channel  string `json:"channel"`
		OrderRef string `json:"orderRef"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.SKU == "" || req.Qty <= 0 {
		return response.BadRequest(c, "SKU and positive quantity are required")
	}

	movement, err := h.purchasingUsecase.CreateGoodsIssue(c.Context(), usecasePurchasing.CreateGoodsIssueInput{
		SKU:      req.SKU,
		Qty:      req.Qty,
		Reason:   req.Reason,
		Channel:  req.Channel,
		OrderRef: req.OrderRef,
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		return response.BadRequest(c, "Failed to create goods issue: "+err.Error())
	}

	return response.Created(c, IssueRecord{
		ID:       movement.ID,
		Code:     fmt.Sprintf("GI-%s-%04d", movement.CreatedAt.Format("2006/01/02"), movement.ID),
		SKU:      movement.SKUCode,
		SKUName:  movement.SKUCode,
		Qty:      movement.Quantity,
		Reason:   movement.Note,
		Date:     movement.CreatedAt.Format("2006-01-02"),
		Channel:  formatMovementChannel(movement.ReferenceType, movement.ReferenceID),
		OrderRef: movement.ReferenceID,
	}, "Goods issued successfully")
}

func (h *WorkspaceHandler) AdjustStock(c *fiber.Ctx) error {
	var req struct {
		Note  string `json:"note"`
		Items []struct {
			SKU       string `json:"sku"`
			ActualQty int    `json:"actualQty"`
		} `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	input := usecaseStock.AdjustBySKUInput{Note: req.Note}
	input.Items = make([]usecaseStock.AdjustBySKULine, len(req.Items))
	for i, it := range req.Items {
		input.Items[i] = usecaseStock.AdjustBySKULine{SKU: it.SKU, ActualQty: it.ActualQty}
	}

	// Business rules (validation, stock-row lock, movement write, transaction)
	// live in the stock usecase — the handler only parses and delegates.
	if err := h.stockUsecase.AdjustBySKU(c.Context(), h.skuRepo, input); err != nil {
		return err
	}

	return response.OK(c, nil, "Stock adjusted successfully")
}

// BundleComponentRecord for frontend compatibility (erp-web-v2 and erp-web)
type BundleComponentRecord struct {
	ID           uint    `json:"id"`
	BundleSKU    string  `json:"bundleSku"`
	ComponentSKU string  `json:"componentSku"`
	Qty          float64 `json:"qty"`
	Note         string  `json:"note"`
}

func (h *WorkspaceHandler) GetBundleComponents(c *fiber.Ctx) error {
	sku := c.Params("sku")

	var items []domainBundle.BundleItem
	var err error
	if sku == "" {
		items, err = h.bundleUsecase.ListComponents(c.Context())
	} else {
		items, err = h.bundleUsecase.GetComponents(c.Context(), strings.ToUpper(strings.TrimSpace(sku)))
	}
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch bundle components: "+err.Error())
	}

	records := make([]BundleComponentRecord, len(items))
	for i, item := range items {
		records[i] = BundleComponentRecord{
			ID:           item.ID,
			BundleSKU:    item.BundleSKU,
			ComponentSKU: item.ComponentSKU,
			Qty:          float64(item.Quantity),
			Note:         item.Note,
		}
	}

	return response.OK(c, records)
}

