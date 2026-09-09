package handler

import (
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecaseStock "chawy-erp-api/internal/usecase/stock"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type StockHandler struct {
	usecase usecaseStock.Usecase
}

func NewStockHandler(usecase usecaseStock.Usecase) *StockHandler {
	return &StockHandler{usecase: usecase}
}

func (h *StockHandler) GetStock(c *fiber.Ctx) error {
	skuID, err := strconv.ParseUint(c.Params("skuId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid SKU ID")
	}

	whID, _ := strconv.ParseUint(c.Query("warehouseId", "1"), 10, 32)

	stock, err := h.usecase.GetStock(c.Context(), uint(skuID), uint(whID))
	if err != nil {
		return err
	}

	return response.OK(c, toStockResponse(stock))
}

func (h *StockHandler) ListStock(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	skuID, _ := strconv.ParseUint(c.Query("skuId", "0"), 10, 32)
	whID, _ := strconv.ParseUint(c.Query("warehouseId", "0"), 10, 32)

	items, total, err := h.usecase.ListStock(c.Context(), domainStock.Query{
		SKUID:       uint(skuID),
		WarehouseID: uint(whID),
		Page:        page,
		Limit:       limit,
	})
	if err != nil {
		return err
	}

	responses := make([]dto.StockResponse, len(items))
	for i, item := range items {
		responses[i] = toStockResponse(&item)
	}

	return response.List(c, responses, page, limit, total)
}

func (h *StockHandler) Adjust(c *fiber.Ctx) error {
	var req dto.AdjustStockRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.SKUID == 0 || req.Quantity <= 0 {
		return response.BadRequest(c, "SKU ID and a positive Quantity are required")
	}

	mType := domainStock.MovementType(req.Type)
	if mType != domainStock.MovementIn && mType != domainStock.MovementOut && mType != domainStock.MovementAdjust {
		return response.BadRequest(c, "Type must be IN, OUT, or ADJUST")
	}

	updatedStock, err := h.usecase.AdjustStock(c.Context(), usecaseStock.AdjustInput{
		SKUID:         req.SKUID,
		SKUCode:       req.SKUCode,
		WarehouseID:   req.WarehouseID,
		Type:          mType,
		Quantity:      req.Quantity,
		ReferenceType: req.ReferenceType,
		ReferenceID:   req.ReferenceID,
		Note:          req.Note,
	})
	if err != nil {
		return err
	}

	return response.OK(c, toStockResponse(updatedStock), "Stock adjusted successfully")
}

func (h *StockHandler) GetMovements(c *fiber.Ctx) error {
	skuID, err := strconv.ParseUint(c.Params("skuId"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid SKU ID")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	movements, total, err := h.usecase.GetMovements(c.Context(), uint(skuID), page, limit)
	if err != nil {
		return err
	}

	return response.List(c, movements, page, limit, total)
}

func toStockResponse(item *domainStock.Stock) dto.StockResponse {
	return dto.StockResponse{
		ID:           item.ID,
		SKUID:        item.SKUID,
		SKUCode:      item.SKUCode,
		WarehouseID:  item.WarehouseID,
		Quantity:     item.Quantity,
		ReservedQty:  item.ReservedQty,
		AvailableQty: item.AvailableQty,
	}
}
