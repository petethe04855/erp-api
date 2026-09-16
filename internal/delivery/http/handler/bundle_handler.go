package handler

import (
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	usecaseBundle "chawy-erp-api/internal/usecase/bundle"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type BundleHandler struct {
	usecase usecaseBundle.Usecase
}

func NewBundleHandler(usecase usecaseBundle.Usecase) *BundleHandler {
	return &BundleHandler{usecase: usecase}
}

func (h *BundleHandler) SetComponents(c *fiber.Ctx) error {
	bundleSKU := c.Params("sku")
	if bundleSKU == "" {
		return response.BadRequest(c, "Bundle SKU is required")
	}

	var req dto.SetBundleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	inputs := make([]usecaseBundle.ComponentInput, len(req.Components))
	for i, comp := range req.Components {
		inputs[i] = usecaseBundle.ComponentInput{
			ComponentSKU: comp.ComponentSKU,
			Quantity:     comp.Quantity,
			Note:         comp.Note,
		}
	}

	if err := h.usecase.SetComponents(c.Context(), bundleSKU, inputs); err != nil {
		return err
	}

	return response.OK(c, nil, "Bundle components updated successfully")
}

func (h *BundleHandler) GetComponents(c *fiber.Ctx) error {
	bundleSKU := c.Params("sku")
	if bundleSKU == "" {
		return response.BadRequest(c, "Bundle SKU is required")
	}

	items, err := h.usecase.GetComponents(c.Context(), bundleSKU)
	if err != nil {
		return err
	}

	return response.OK(c, items)
}

func (h *BundleHandler) Explode(c *fiber.Ctx) error {
	bundleSKU := c.Params("sku")
	if bundleSKU == "" {
		return response.BadRequest(c, "Bundle SKU is required")
	}

	qty, _ := strconv.Atoi(c.Query("qty", "1"))
	whID, _ := strconv.ParseUint(c.Query("warehouseId", "1"), 10, 32)

	exploded, allInStock, err := h.usecase.ExplodeBundle(c.Context(), bundleSKU, qty, uint(whID))
	if err != nil {
		return err
	}

	compDTOs := make([]dto.ExplodedComponentDTO, len(exploded))
	for i, itm := range exploded {
		compDTOs[i] = dto.ExplodedComponentDTO{
			ComponentSKU: itm.ComponentSKU,
			Quantity:     itm.Quantity,
			AvailableQty: itm.AvailableQty,
			HasStock:     itm.HasStock,
		}
	}

	res := dto.ExplodeResponse{
		BundleSKU:  bundleSKU,
		OrderQty:   qty,
		AllInStock: allInStock,
		Components: compDTOs,
	}

	return response.OK(c, res)
}
