package handler

import (
	"strconv"

	usecaseSKU "chawy-erp-api/internal/usecase/sku"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type SKUCostHistoryHandler struct {
	usecase usecaseSKU.CostHistoryUsecase
}

func NewSKUCostHistoryHandler(usecase usecaseSKU.CostHistoryUsecase) *SKUCostHistoryHandler {
	return &SKUCostHistoryHandler{usecase: usecase}
}

type addCostHistoryReq struct {
	CostPrice     float64 `json:"cost_price"`
	EffectiveFrom string  `json:"effective_from"`
	EffectiveTo   *string `json:"effective_to"`
	Note          string  `json:"note"`
}

// AddCostHistory adds a new date-effective cost
func (h *SKUCostHistoryHandler) AddCostHistory(c *fiber.Ctx) error {
	skuCode := c.Params("sku")
	if skuCode == "" {
		return response.BadRequest(c, "SKU parameter is required")
	}

	var req addCostHistoryReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}

	res, err := h.usecase.AddCostHistory(c.Context(), skuCode, req.CostPrice, req.EffectiveFrom, req.EffectiveTo, req.Note)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Created(c, res)
}

// GetCostHistory returns all cost records for an SKU
func (h *SKUCostHistoryHandler) GetCostHistory(c *fiber.Ctx) error {
	skuCode := c.Params("sku")
	if skuCode == "" {
		return response.BadRequest(c, "SKU parameter is required")
	}

	list, err := h.usecase.GetCostHistoryBySKU(c.Context(), skuCode)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, list)
}

// DeleteCostHistory deletes a cost history record
func (h *SKUCostHistoryHandler) DeleteCostHistory(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid cost history ID")
	}

	if err := h.usecase.DeleteCostHistory(c.Context(), uint(id)); err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, map[string]string{"message": "Cost history deleted successfully"})
}
