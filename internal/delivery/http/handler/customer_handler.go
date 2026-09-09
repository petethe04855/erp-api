package handler

import (
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	domainCustomer "chawy-erp-api/internal/domain/customer"
	usecaseCustomer "chawy-erp-api/internal/usecase/customer"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type CustomerHandler struct {
	usecase usecaseCustomer.Usecase
}

func NewCustomerHandler(usecase usecaseCustomer.Usecase) *CustomerHandler {
	return &CustomerHandler{usecase: usecase}
}

func (h *CustomerHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Name == "" {
		return response.BadRequest(c, "Customer name is required")
	}

	contactPerson := req.ContactPerson
	if contactPerson == "" && req.ContactPersonAlt != "" {
		contactPerson = req.ContactPersonAlt
	}

	taxID := req.TaxID
	if taxID == "" && req.TaxIDAlt != "" {
		taxID = req.TaxIDAlt
	}

	result, err := h.usecase.Create(c.Context(), usecaseCustomer.CreateInput{
		Code:          req.Code,
		Name:          req.Name,
		ContactPerson: contactPerson,
		Phone:         req.Phone,
		Email:         req.Email,
		Address:       req.Address,
		TaxID:         taxID,
		Channel:       req.Channel,
	})
	if err != nil {
		return err
	}

	return response.Created(c, result, "Customer created successfully")
}

func (h *CustomerHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}

	result, err := h.usecase.GetByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, result)
}

func (h *CustomerHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")
	channel := c.Query("channel", "")
	status := c.Query("status", "")

	items, total, err := h.usecase.List(c.Context(), domainCustomer.Query{
		Search:  search,
		Channel: channel,
		Status:  status,
		Page:    page,
		Limit:   limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, items, page, limit, total)
}

func (h *CustomerHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}

	var req dto.UpdateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	contactPerson := req.ContactPerson
	if contactPerson == "" && req.ContactPersonAlt != "" {
		contactPerson = req.ContactPersonAlt
	}

	taxID := req.TaxID
	if taxID == "" && req.TaxIDAlt != "" {
		taxID = req.TaxIDAlt
	}

	result, err := h.usecase.Update(c.Context(), uint(id), usecaseCustomer.UpdateInput{
		Name:          req.Name,
		ContactPerson: contactPerson,
		Phone:         req.Phone,
		Email:         req.Email,
		Address:       req.Address,
		TaxID:         taxID,
		Channel:       req.Channel,
		Status:        req.Status,
	})
	if err != nil {
		return err
	}

	return response.OK(c, result, "Customer updated successfully")
}

func (h *CustomerHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}

	if err := h.usecase.Delete(c.Context(), uint(id)); err != nil {
		return err
	}

	return response.OK(c, nil, "Customer deleted successfully")
}
