package handler

import (
	"strconv"
	"strings"

	domainCustomer "chawy-erp-api/internal/domain/customer"
	usecaseCustomer "chawy-erp-api/internal/usecase/customer"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// CustomerRecord matching erp-web-v2 features/erp/types/records.ts
type CustomerRecord struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContactPerson string `json:"contactPerson"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	TaxID         string `json:"taxId"`
	Address       string `json:"address"`
	Province      string `json:"province"`
	Logo          string `json:"logo"`
}

func (h *WorkspaceHandler) GetCustomers(c *fiber.Ctx) error {
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

	// The customer usecase keeps the workspace query semantics (search also
	// matches contact_person); count/scan errors stay ignored like before.
	customers, total, _ := h.customerUsecase.ListForWorkspace(c.Context(), domainCustomer.WorkspaceQuery{Page: page, Limit: limit, Search: search})

	records := make([]CustomerRecord, len(customers))
	for i, cust := range customers {
		cp := cust.ContactPerson
		if cp == "" {
			cp = cust.Name
		}
		records[i] = CustomerRecord{
			ID:            strconv.Itoa(int(cust.ID)),
			Name:          cust.Name,
			ContactPerson: cp,
			Email:         cust.Email,
			Phone:         cust.Phone,
			TaxID:         cust.TaxID,
			Address:       cust.Address,
			Province:      cust.Province,
			Logo:          cust.Logo,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) CreateCustomer(c *fiber.Ctx) error {
	var req struct {
		Name          string `json:"name"`
		ContactPerson string `json:"contactPerson"`
		Email         string `json:"email"`
		Phone         string `json:"phone"`
		TaxID         string `json:"taxId"`
		Address       string `json:"address"`
		Logo          string `json:"logo"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	cust, err := h.customerUsecase.CreateForWorkspace(c.Context(), usecaseCustomer.WorkspaceCreateInput{
		Name:          req.Name,
		ContactPerson: req.ContactPerson,
		Email:         req.Email,
		Phone:         req.Phone,
		TaxID:         req.TaxID,
		Address:       req.Address,
		Logo:          req.Logo,
	})
	if err != nil {
		return err
	}

	rec := CustomerRecord{
		ID:            strconv.Itoa(int(cust.ID)),
		Name:          cust.Name,
		ContactPerson: cust.ContactPerson,
		Email:         cust.Email,
		Phone:         cust.Phone,
		TaxID:         cust.TaxID,
		Address:       cust.Address,
		Logo:          cust.Logo,
	}
	return response.Created(c, rec, "Customer created successfully")
}

func (h *WorkspaceHandler) GetCustomerByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}

	cust, err := h.customerUsecase.GetByID(c.Context(), uint(id))
	if err != nil {
		return response.NotFound(c, "Customer not found")
	}

	cp := cust.ContactPerson
	if cp == "" {
		cp = cust.Name
	}

	return response.OK(c, fiber.Map{
		"id":            cust.ID,
		"code":          cust.Code,
		"name":          cust.Name,
		"contactPerson": cp,
		"phone":         cust.Phone,
		"email":         cust.Email,
		"address":       cust.Address,
		"taxId":         cust.TaxID,
		"logo":          cust.Logo,
		"channel":       cust.Channel,
		"status":        cust.Status,
		"createdAt":     cust.CreatedAt.Format("2006-01-02 15:04"),
	})
}

func (h *WorkspaceHandler) UpdateCustomerStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	cust, err := h.customerUsecase.SetStatusForWorkspace(c.Context(), uint(id), req.Status)
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return respondAppError(c, appErr)
		}
		return response.InternalServerError(c, "Failed to update customer status")
	}

	return response.OK(c, cust, "Customer status updated")
}

