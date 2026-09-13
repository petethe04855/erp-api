package handler

import (
	"strconv"
	"strings"

	domainFormula "chawy-erp-api/internal/domain/formula"
	usecaseFormula "chawy-erp-api/internal/usecase/formula"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type FormulaHandler struct {
	usecase usecaseFormula.Usecase
}

func NewFormulaHandler(usecase usecaseFormula.Usecase) *FormulaHandler {
	return &FormulaHandler{usecase: usecase}
}

func (h *FormulaHandler) Create(c *fiber.Ctx) error {
	var req struct {
		Code        string                     `json:"code"`
		Name        string                     `json:"name"`
		Description string                     `json:"description"`
		IsActive    *bool                      `json:"isActive"`
		Items       []usecaseFormula.ItemInput `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	result, err := h.usecase.Create(c.Context(), usecaseFormula.CreateFormulaInput{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
		Items:       req.Items,
	})
	if err != nil {
		return err
	}

	return response.Created(c, result, "สร้างสูตรตัดสต็อกเรียบร้อยแล้ว")
}

func (h *FormulaHandler) Update(c *fiber.Ctx) error {
	code := c.Params("code")
	if strings.TrimSpace(code) == "" {
		return response.BadRequest(c, "Formula code is required")
	}

	var req struct {
		Name        *string                    `json:"name"`
		Description *string                    `json:"description"`
		IsActive    *bool                      `json:"isActive"`
		Items       []usecaseFormula.ItemInput `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	result, err := h.usecase.Update(c.Context(), code, usecaseFormula.UpdateFormulaInput{
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
		Items:       req.Items,
	})
	if err != nil {
		return err
	}

	return response.OK(c, result, "แก้ไขสูตรตัดสต็อกเรียบร้อยแล้ว")
}

func (h *FormulaHandler) ToggleStatus(c *fiber.Ctx) error {
	code := c.Params("code")
	if strings.TrimSpace(code) == "" {
		return response.BadRequest(c, "Formula code is required")
	}

	var req struct {
		IsActive bool `json:"isActive"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	result, err := h.usecase.ToggleStatus(c.Context(), code, req.IsActive)
	if err != nil {
		return err
	}

	return response.OK(c, result, "อัปเดตสถานะสูตรตัดสต็อกเรียบร้อยแล้ว")
}

func (h *FormulaHandler) Deactivate(c *fiber.Ctx) error {
	code := c.Params("code")
	if strings.TrimSpace(code) == "" {
		return response.BadRequest(c, "Formula code is required")
	}

	if err := h.usecase.Deactivate(c.Context(), code); err != nil {
		return err
	}

	return response.OK(c, nil, "ปิดใช้งานสูตรตัดสต็อกเรียบร้อยแล้ว")
}

func (h *FormulaHandler) GetByCode(c *fiber.Ctx) error {
	code := c.Params("code")
	if strings.TrimSpace(code) == "" {
		return response.BadRequest(c, "Formula code is required")
	}

	result, err := h.usecase.GetByCode(c.Context(), code)
	if err != nil {
		return err
	}

	return response.OK(c, result)
}

func (h *FormulaHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	search := c.Query("search", "")
	status := c.Query("status", "")

	var isActive *bool
	if strings.EqualFold(status, "active") {
		act := true
		isActive = &act
	} else if strings.EqualFold(status, "inactive") {
		act := false
		isActive = &act
	}

	items, total, err := h.usecase.List(c.Context(), domainFormula.Query{
		Search:   search,
		IsActive: isActive,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, items, page, limit, total)
}
