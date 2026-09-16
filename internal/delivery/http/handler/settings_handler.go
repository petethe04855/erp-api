package handler

import (
	"chawy-erp-api/internal/delivery/http/dto"
	usecaseSettings "chawy-erp-api/internal/usecase/settings"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type SettingsHandler struct {
	usecase usecaseSettings.Usecase
}

func NewSettingsHandler(usecase usecaseSettings.Usecase) *SettingsHandler {
	return &SettingsHandler{usecase: usecase}
}

func (h *SettingsHandler) GetSettings(c *fiber.Ctx) error {
	res, err := h.usecase.GetSettings(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, res)
}

func (h *SettingsHandler) UpdateSettings(c *fiber.Ctx) error {
	var req dto.UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	res, err := h.usecase.UpdateSettings(c.Context(), usecaseSettings.UpdateSettingsInput{
		Company:       req.Company,
		Notifications: req.Notifications,
		Modules:       req.Modules,
		LivePayroll:   req.LivePayroll,
	})
	if err != nil {
		return err
	}

	return response.OK(c, res, "Settings updated successfully")
}
