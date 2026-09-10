package handler

import (
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseSKU "chawy-erp-api/internal/usecase/sku"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type SKUHandler struct {
	usecase usecaseSKU.Usecase
}

func NewSKUHandler(usecase usecaseSKU.Usecase) *SKUHandler {
	return &SKUHandler{usecase: usecase}
}

func (h *SKUHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateSKURequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.SKU == "" || req.Name == "" {
		return response.BadRequest(c, "SKU and Name are required fields")
	}

	result, err := h.usecase.Create(c.Context(), usecaseSKU.CreateInput{
		SKU:       req.SKU,
		Name:      req.Name,
		Barcode:   req.Barcode,
		Category:  req.Category,
		Price:     req.Price,
		CostPrice: req.CostPrice,
		IsBundle:  req.IsBundle,
		Image:     req.Image,
	})
	if err != nil {
		return err
	}

	return response.Created(c, toSKUResponse(result), "SKU created successfully")
}

func (h *SKUHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid SKU ID")
	}

	result, err := h.usecase.GetByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, toSKUResponse(result))
}

func (h *SKUHandler) GetBySKU(c *fiber.Ctx) error {
	skuCode := c.Params("code")
	if skuCode == "" {
		return response.BadRequest(c, "SKU code is required")
	}

	result, err := h.usecase.GetBySKU(c.Context(), skuCode)
	if err != nil {
		return err
	}

	return response.OK(c, toSKUResponse(result))
}

func (h *SKUHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")
	category := c.Query("category", "")
	status := c.Query("status", "")

	items, total, err := h.usecase.List(c.Context(), domainSKU.Query{
		Search:   search,
		Category: category,
		Status:   status,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		return err
	}

	responses := make([]dto.SKUResponse, len(items))
	for i, item := range items {
		responses[i] = toSKUResponse(&item)
	}

	return response.List(c, responses, page, limit, total)
}

func (h *SKUHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid SKU ID")
	}

	var req dto.UpdateSKURequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	result, err := h.usecase.Update(c.Context(), uint(id), usecaseSKU.UpdateInput{
		Name:      req.Name,
		Barcode:   req.Barcode,
		Category:  req.Category,
		Price:     req.Price,
		CostPrice: req.CostPrice,
		IsBundle:  req.IsBundle,
		Image:     req.Image,
		Status:    req.Status,
	})
	if err != nil {
		return err
	}

	return response.OK(c, toSKUResponse(result), "SKU updated successfully")
}

func (h *SKUHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid SKU ID")
	}

	if err := h.usecase.Delete(c.Context(), uint(id)); err != nil {
		return err
	}

	return response.OK(c, nil, "SKU deleted successfully")
}

func toSKUResponse(item *domainSKU.SKU) dto.SKUResponse {
	return dto.SKUResponse{
		ID:        item.ID,
		SKU:       item.SKU,
		Name:      item.Name,
		Barcode:   item.Barcode,
		Category:  item.Category,
		Price:     item.Price,
		CostPrice: item.CostPrice,
		IsBundle:  item.IsBundle,
		Image:     item.Image,
		Status:    item.Status,
	}
}
