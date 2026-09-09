package handler

import (
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	domainOrder "chawy-erp-api/internal/domain/order"
	usecaseOrder "chawy-erp-api/internal/usecase/order"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type OrderHandler struct {
	usecase usecaseOrder.Usecase
}

func NewOrderHandler(usecase usecaseOrder.Usecase) *OrderHandler {
	return &OrderHandler{usecase: usecase}
}

func (h *OrderHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if len(req.Items) == 0 {
		return response.BadRequest(c, "Order must have at least one item")
	}

	itemInputs := make([]usecaseOrder.CreateItemInput, len(req.Items))
	for i, item := range req.Items {
		if item.Quantity <= 0 {
			item.Quantity = 1
		}
		itemInputs[i] = usecaseOrder.CreateItemInput{
			SKU:      item.SKU,
			Quantity: item.Quantity,
			Price:    item.Price,
		}
	}

	result, err := h.usecase.Create(c.Context(), usecaseOrder.CreateOrderInput{
		CustomerID:   req.CustomerID,
		CustomerName: req.CustomerName,
		Channel:      req.Channel,
		Note:         req.Note,
		Items:        itemInputs,
	})
	if err != nil {
		return err
	}

	return response.Created(c, result, "Order created successfully")
}

func (h *OrderHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	result, err := h.usecase.GetByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, result)
}

func (h *OrderHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")
	status := c.Query("status", "")
	channel := c.Query("channel", "")

	items, total, err := h.usecase.List(c.Context(), domainOrder.Query{
		Search:  search,
		Status:  status,
		Channel: channel,
		Page:    page,
		Limit:   limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, items, page, limit, total)
}

func (h *OrderHandler) Ship(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	var req dto.ShipOrderRequest
	_ = c.BodyParser(&req)
	if req.WarehouseID == 0 {
		req.WarehouseID = 1
	}

	result, err := h.usecase.ShipOrder(c.Context(), uint(id), req.WarehouseID)
	if err != nil {
		return err
	}

	return response.OK(c, result, "Order shipped and inventory deducted successfully")
}

func (h *OrderHandler) Cancel(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	result, err := h.usecase.CancelOrder(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, result, "Order cancelled successfully")
}
