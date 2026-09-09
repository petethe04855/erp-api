package handler

import (
	usecaseReport "chawy-erp-api/internal/usecase/report"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type ReportHandler struct {
	usecase usecaseReport.Usecase
}

func NewReportHandler(usecase usecaseReport.Usecase) *ReportHandler {
	return &ReportHandler{usecase: usecase}
}

func (h *ReportHandler) GetDashboard(c *fiber.Ctx) error {
	summary, err := h.usecase.GetDashboardSummary(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, summary)
}

func (h *ReportHandler) GetRevenue(c *fiber.Ctx) error {
	month := c.Query("month", "")
	res, err := h.usecase.GetRevenueReport(c.Context(), month)
	if err != nil {
		return err
	}
	return response.OK(c, res)
}

func (h *ReportHandler) GetFinancialSummary(c *fiber.Ctx) error {
	month := c.Query("month", "")
	res, err := h.usecase.GetFinancialSummary(c.Context(), month)
	if err != nil {
		return err
	}
	return response.OK(c, res)
}

func (h *ReportHandler) GetInventoryValuation(c *fiber.Ctx) error {
	res, err := h.usecase.GetInventoryValuation(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, res)
}
