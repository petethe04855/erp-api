package handler

import (
	"errors"
	"strconv"
	"time"

	usecaseShopee "chawy-erp-api/internal/usecase/shopee"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ShopeeInsightHandler serves the Shopee sales insights (by order date).
type ShopeeInsightHandler struct {
	uc usecaseShopee.InsightUsecase
}

func NewShopeeInsightHandler(uc usecaseShopee.InsightUsecase) *ShopeeInsightHandler {
	return &ShopeeInsightHandler{uc: uc}
}

func (h *ShopeeInsightHandler) rangeOf(c *fiber.Ctx) (usecaseShopee.InsightRange, error) {
	return usecaseShopee.ParseInsightRange(c.Query("from"), c.Query("to"), time.Now())
}

func (h *ShopeeInsightHandler) respond(c *fiber.Ctx, res any, err error) error {
	if err != nil {
		if errors.Is(err, usecaseShopee.ErrInvalidInsightRange) {
			return response.BadRequest(c, err.Error())
		}
		return response.InternalServerError(c, err.Error())
	}
	return response.OK(c, res)
}

// Summary: KPIs and sales/orders per month. GET /shopee/insights/summary?from=YYYY-MM&to=YYYY-MM
func (h *ShopeeInsightHandler) Summary(c *fiber.Ctx) error {
	rng, err := h.rangeOf(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	res, err := h.uc.Summary(c.Context(), rng)
	return h.respond(c, res, err)
}

// Products: sales by category per month and the product table.
func (h *ShopeeInsightHandler) Products(c *fiber.Ctx) error {
	rng, err := h.rangeOf(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	res, err := h.uc.Products(c.Context(), rng)
	return h.respond(c, res, err)
}

// Sets: freeze-dried set sizes per month and bags per order.
func (h *ShopeeInsightHandler) Sets(c *fiber.Ctx) error {
	rng, err := h.rangeOf(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	res, err := h.uc.Sets(c.Context(), rng)
	return h.respond(c, res, err)
}

// Customers: customers by number of purchases and days until the second purchase.
func (h *ShopeeInsightHandler) Customers(c *fiber.Ctx) error {
	rng, err := h.rangeOf(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	res, err := h.uc.Customers(c.Context(), rng)
	return h.respond(c, res, err)
}

// Journey: what customers buy on their 1st to Nth order (Sankey). ?steps=2..6 (default 4)
func (h *ShopeeInsightHandler) Journey(c *fiber.Ctx) error {
	rng, err := h.rangeOf(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}
	steps := 4
	if s := c.Query("steps"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 2 || n > 6 {
			return response.BadRequest(c, "steps must be between 2 and 6")
		}
		steps = n
	}
	res, err := h.uc.Journey(c.Context(), rng, steps)
	return h.respond(c, res, err)
}
