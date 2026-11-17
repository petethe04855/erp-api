package handler

import (
	"errors"

	"chawy-erp-api/internal/domain/salesanalytics"
	usecaseSA "chawy-erp-api/internal/usecase/salesanalytics"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// SalesAnalyticsHandler serves the CRM sales dashboard facts endpoint.
type SalesAnalyticsHandler struct {
	usecase usecaseSA.Usecase
}

// NewSalesAnalyticsHandler builds the handler.
func NewSalesAnalyticsHandler(usecase usecaseSA.Usecase) *SalesAnalyticsHandler {
	return &SalesAnalyticsHandler{usecase: usecase}
}

// GetSalesAnalytics handles GET /api/v1/crm/sales-analytics.
//
//	?dateFrom=2026-08-19&dateTo=2026-11-17  inclusive Bangkok days
//	&channel=all|shopee|tiktok              optional
//	&region=<Thai region name>              optional
func (h *SalesAnalyticsHandler) GetSalesAnalytics(c *fiber.Ctx) error {
	payload, err := h.usecase.SalesAnalytics(c.Context(), salesanalytics.Query{
		DateFrom: c.Query("dateFrom"),
		DateTo:   c.Query("dateTo"),
		Channel:  c.Query("channel"),
		Region:   c.Query("region"),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecaseSA.ErrInvalidDateFormat),
			errors.Is(err, usecaseSA.ErrDateFromAfterDateTo),
			errors.Is(err, usecaseSA.ErrDateRangeExceeded),
			errors.Is(err, usecaseSA.ErrInvalidChannel),
			errors.Is(err, usecaseSA.ErrInvalidRegion):
			return response.BadRequest(c, err.Error(), "VALIDATION_ERROR")
		default:
			return response.InternalServerError(c, "Failed to load sales analytics")
		}
	}
	return response.OK(c, payload)
}
