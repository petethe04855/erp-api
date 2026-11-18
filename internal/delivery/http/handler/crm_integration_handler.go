package handler

import (
	"errors"
	"strconv"
	"strings"

	"chawy-erp-api/internal/delivery/http/dto"
	usecaseCRMIntegration "chawy-erp-api/internal/usecase/crmintegration"
	appErrors "chawy-erp-api/pkg/errors"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// CRMIntegrationHandler serves /api/v1/integrations/crm for backoffice-api.
// The routes are authenticated with the CRM service token, not a staff JWT.
type CRMIntegrationHandler struct {
	uc usecaseCRMIntegration.Usecase
}

// NewCRMIntegrationHandler builds the handler.
func NewCRMIntegrationHandler(uc usecaseCRMIntegration.Usecase) *CRMIntegrationHandler {
	return &CRMIntegrationHandler{uc: uc}
}

func queryUint(c *fiber.Ctx, key string) (uint, error) {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, appErrors.NewAppError("VALIDATION_ERROR", key+" must be a positive integer", 400)
	}
	return uint(n), nil
}

func (h *CRMIntegrationHandler) fail(c *fiber.Ctx, err error) error {
	var short *usecaseCRMIntegration.InsufficientStockError
	if errors.As(err, &short) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"error":   fiber.Map{"code": "INSUFFICIENT_STOCK", "message": short.Error()},
			"data":    fiber.Map{"shortages": short.Shortages},
		})
	}
	var appErr *appErrors.AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.StatusCode, appErr.Code, appErr.Message)
	}
	return response.InternalServerError(c, "CRM integration failed")
}

// SearchSKUs serves GET /integrations/crm/skus?search=&page=&limit=&warehouseId=.
func (h *CRMIntegrationHandler) SearchSKUs(c *fiber.Ctx) error {
	warehouse, err := queryUint(c, "warehouseId")
	if err != nil {
		return h.fail(c, err)
	}
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	items, total, err := h.uc.SearchSKUs(c.UserContext(), c.Query("search"), warehouse, page, limit)
	if err != nil {
		return h.fail(c, err)
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return response.List(c, items, page, limit, total)
}

// Stock serves GET /integrations/crm/stock?skuIds=1,2,3&warehouseId=.
func (h *CRMIntegrationHandler) Stock(c *fiber.Ctx) error {
	warehouse, err := queryUint(c, "warehouseId")
	if err != nil {
		return h.fail(c, err)
	}
	var ids []uint
	for _, part := range strings.Split(c.Query("skuIds"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil || n == 0 {
			return h.fail(c, appErrors.NewAppError("VALIDATION_ERROR", "skuIds must be positive integers separated by commas", 400))
		}
		ids = append(ids, uint(n))
	}
	items, err := h.uc.Stock(c.UserContext(), ids, warehouse)
	if err != nil {
		return h.fail(c, err)
	}
	return response.OK(c, items)
}

// CreateGoodsIssue serves POST /integrations/crm/goods-issues. A replayed
// idempotency key answers 200 with the first giRef; a new issue answers 201.
func (h *CRMIntegrationHandler) CreateGoodsIssue(c *fiber.Ctx) error {
	var req dto.CRMGoodsIssueRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid JSON body", "VALIDATION_ERROR")
	}
	in := usecaseCRMIntegration.GoodsIssueInput{
		IdempotencyKey: req.IdempotencyKey, WarehouseID: req.WarehouseID,
		Reference: req.Reference, Note: req.Note,
	}
	for _, it := range req.Items {
		in.Items = append(in.Items, usecaseCRMIntegration.GoodsIssueLine{SKUID: it.SKUID, Qty: it.Qty})
	}
	res, err := h.uc.CreateGoodsIssue(c.UserContext(), in)
	if err != nil {
		return h.fail(c, err)
	}
	if res.Replayed {
		return response.OK(c, res, "Goods issue already recorded")
	}
	return response.Created(c, res, "Goods issued")
}
