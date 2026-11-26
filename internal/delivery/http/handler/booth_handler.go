package handler

import (
	"errors"
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	usecaseBooth "chawy-erp-api/internal/usecase/booth"
	appErrors "chawy-erp-api/pkg/errors"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// BoothHandler serves /api/v1/booth for the booth sell screen.
type BoothHandler struct {
	uc usecaseBooth.Usecase
}

// NewBoothHandler builds the handler.
func NewBoothHandler(uc usecaseBooth.Usecase) *BoothHandler {
	return &BoothHandler{uc: uc}
}

func boothBill(b *usecaseBooth.Bill) dto.BoothBillDTO {
	return dto.BoothBillFromParts(b.Bags, b.Lines, b.TreatParts, b.TreatLabel, b.TreatTotal, b.ItemsTotal, b.Subtotal, b.Discount, b.Total)
}

type boothButtonDTO struct {
	SKUID     uint    `json:"skuId"`
	SKUCode   string  `json:"skuCode"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Color     string  `json:"color,omitempty"`
	Price     float64 `json:"price,omitempty"`
	Available int     `json:"available"`
	Missing   bool    `json:"missing,omitempty"`
}

func boothCatalog(c *usecaseBooth.Catalog) fiber.Map {
	buttons := make([]boothButtonDTO, 0, len(c.Buttons))
	for _, b := range c.Buttons {
		buttons = append(buttons, boothButtonDTO(b))
	}
	return fiber.Map{"config": dto.BoothConfigFromDomain(c.Config), "buttons": buttons, "updatedAt": c.UpdatedAt, "updatedBy": c.UpdatedBy}
}

func (h *BoothHandler) fail(c *fiber.Ctx, err error) error {
	var short *usecaseBooth.InsufficientStockError
	if errors.As(err, &short) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   fiber.Map{"code": "INSUFFICIENT_STOCK", "message": short.Error()},
			"data":    fiber.Map{"shortages": short.Shortages},
		})
	}
	var changed *usecaseBooth.PriceChangedError
	if errors.As(err, &changed) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   fiber.Map{"code": "PRICE_CHANGED", "message": changed.Error()},
			"data":    fiber.Map{"bill": boothBill(changed.Bill)},
		})
	}
	var appErr *appErrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.StatusCode, appErr.Code, appErr.Message)
	}
	return response.InternalServerError(c, "Booth request failed")
}

func cartLines(lines []dto.BoothCartLine) []usecaseBooth.CartLine {
	out := make([]usecaseBooth.CartLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, usecaseBooth.CartLine{SKUID: l.SKUID, Qty: l.Qty})
	}
	return out
}

// GetCatalog serves GET /booth/catalog: buttons, prices and stock left.
func (h *BoothHandler) GetCatalog(c *fiber.Ctx) error {
	cat, err := h.uc.GetCatalog(c.UserContext())
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, boothCatalog(cat))
}

// UpdateConfig serves PUT /booth/config (owner).
func (h *BoothHandler) UpdateConfig(c *fiber.Ctx) error {
	var req dto.BoothConfigDTO
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	actor, _ := c.Locals("email").(string)
	cat, err := h.uc.UpdateConfig(c.UserContext(), req.ToDomain(), actor)
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, boothCatalog(cat), "Booth settings saved")
}

// Quote serves POST /booth/quote: the price of a cart, nothing saved.
func (h *BoothHandler) Quote(c *fiber.Ctx) error {
	var req dto.BoothQuoteRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	bill, err := h.uc.Quote(c.UserContext(), cartLines(req.Lines))
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, boothBill(bill))
}

// CreateSale serves POST /booth/sales. A replayed clientRef answers 200 with
// the first bill; a new bill answers 201.
func (h *BoothHandler) CreateSale(c *fiber.Ctx) error {
	var req dto.BoothSaleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	userID, _ := c.Locals("userID").(uint)
	email, _ := c.Locals("email").(string)
	res, err := h.uc.CreateSale(c.UserContext(), usecaseBooth.SaleInput{
		ClientRef: req.ClientRef, Lines: cartLines(req.Lines), PaymentMethod: req.PaymentMethod,
		CashReceived: req.CashReceived, ExpectedTotal: req.ExpectedTotal, SellerID: userID, SellerName: email,
	})
	if err != nil {
		return h.fail(c, err)
	}
	body := fiber.Map{"sale": dto.BoothSaleFromDomain(res.Sale), "replayed": res.Replayed}
	if res.Replayed {
		return response.OK(c, body, "Bill already recorded")
	}
	return response.Created(c, body, "Bill recorded")
}

// VoidSale serves POST /booth/sales/:id/void.
func (h *BoothHandler) VoidSale(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || id == 0 {
		return response.BadRequest(c, "Invalid bill id", "VALIDATION_ERROR")
	}
	var req dto.BoothVoidRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
		}
	}
	actor, _ := c.Locals("email").(string)
	sale, err := h.uc.VoidSale(c.UserContext(), uint(id), actor, req.Reason)
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, dto.BoothSaleFromDomain(sale), "Bill voided")
}

// ListSales serves GET /booth/sales?date=YYYY-MM-DD (today when empty).
func (h *BoothHandler) ListSales(c *fiber.Ctx) error {
	sales, err := h.uc.ListSales(c.UserContext(), c.Query("date"))
	if err != nil {
		return h.fail(c, err)
	}
	out := make([]dto.BoothSaleDTO, 0, len(sales))
	for i := range sales {
		out = append(out, dto.BoothSaleFromDomain(&sales[i]))
	}
	return response.OK(c, out)
}
