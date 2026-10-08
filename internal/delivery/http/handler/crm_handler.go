package handler

import (
	"errors"

	"chawy-erp-api/internal/domain/crm"
	usecaseCRM "chawy-erp-api/internal/usecase/crm"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type CRMHandler struct {
	crmUsecase usecaseCRM.Usecase
}

func NewCRMHandler(crmUsecase usecaseCRM.Usecase) *CRMHandler {
	return &CRMHandler{crmUsecase: crmUsecase}
}

// GetTiktokProvinceReport handles GET /api/v1/crm/tiktok/provinces
func (h *CRMHandler) GetTiktokProvinceReport(c *fiber.Ctx) error {
	query := crm.ProvinceQuery{
		DateFrom: c.Query("dateFrom"),
		DateTo:   c.Query("dateTo"),
		Status:   c.Query("status"),
		Province: c.Query("province"),
		Channel:  c.Query("channel"),
	}

	report, err := h.crmUsecase.GetTiktokProvinceReport(c.Context(), query)
	if err != nil {
		if errors.Is(err, usecaseCRM.ErrDateFromAfterDateTo) ||
			errors.Is(err, usecaseCRM.ErrDateRangeExceeded) ||
			errors.Is(err, usecaseCRM.ErrInvalidDateFormat) {
			return response.BadRequest(c, err.Error(), "INVALID_QUERY_PARAMS")
		}
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, report)
}

func (h *CRMHandler) SearchProvinceReport(c *fiber.Ctx) error {
	var req crm.ProvinceSearchRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	report, err := h.crmUsecase.SearchProvinceReport(c.Context(), req)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, report)
}
