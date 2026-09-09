package handler

import (
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	usecaseInvoice "chawy-erp-api/internal/usecase/invoice"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type InvoiceHandler struct {
	usecase usecaseInvoice.Usecase
}

func NewInvoiceHandler(usecase usecaseInvoice.Usecase) *InvoiceHandler {
	return &InvoiceHandler{usecase: usecase}
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
