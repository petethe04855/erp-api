package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
	usecaseShopee "chawy-erp-api/internal/usecase/shopee"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type ShopeeHandler struct {
	orderUsecase     usecaseShopee.OrderUsecase
	incomeUsecase    usecaseShopee.IncomeUsecase
	matchingUsecase  usecaseShopee.MatchingUsecase
	dashboardUsecase usecaseShopee.DashboardUsecase
}

func NewShopeeHandler(
	orderUsecase usecaseShopee.OrderUsecase,
	incomeUsecase usecaseShopee.IncomeUsecase,
	matchingUsecase usecaseShopee.MatchingUsecase,
	dashboardUsecase usecaseShopee.DashboardUsecase,
) *ShopeeHandler {
	return &ShopeeHandler{
		orderUsecase:     orderUsecase,
		incomeUsecase:    incomeUsecase,
		matchingUsecase:  matchingUsecase,
		dashboardUsecase: dashboardUsecase,
	}
}

// PreviewOrders parses uploaded orders file and returns preview
func (h *ShopeeHandler) PreviewOrders(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "Please upload a file (.xlsx, .xls, .csv)")
	}

	f, err := file.Open()
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to open file: %v", err))
	}
	defer f.Close()

	res, err := h.orderUsecase.PreviewOrderFile(c.Context(), f, file.Filename)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.OK(c, res)
}

// ImportOrders parses uploaded orders file and commits to DB
func (h *ShopeeHandler) ImportOrders(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "Please upload a file (.xlsx, .xls, .csv)")
	}

	f, err := file.Open()
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to open file: %v", err))
	}
	defer f.Close()

	res, err := h.orderUsecase.ImportOrderFile(c.Context(), f, file.Filename)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.OK(c, res)
}

// GetProvinces returns distinct list of provinces from shopee orders
func (h *ShopeeHandler) GetProvinces(c *fiber.Ctx) error {
	provinces, err := h.orderUsecase.GetProvinces(c.Context())
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.OK(c, provinces)
}

// GetOrders returns paginated order list
func (h *ShopeeHandler) GetOrders(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	search := strings.TrimSpace(c.Query("search"))
	province := strings.TrimSpace(c.Query("province"))
	if province == "" {
		province = strings.TrimSpace(c.Query("provider"))
	}
	startDateStr := strings.TrimSpace(c.Query("start_date"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))

	if c.Method() == fiber.MethodPost {
		var body struct {
			Page       int    `json:"page"`
			Limit      int    `json:"limit"`
			Search     string `json:"search"`
			Province   string `json:"province"`
			Provider   string `json:"provider"`
			StartDate  string `json:"start_date"`
			StartDate2 string `json:"startDate"`
			EndDate    string `json:"end_date"`
			EndDate2   string `json:"endDate"`
		}
		if err := c.BodyParser(&body); err == nil {
			if body.Page > 0 {
				page = body.Page
			}
			if body.Limit > 0 {
				limit = body.Limit
			}
			if strings.TrimSpace(body.Search) != "" {
				search = strings.TrimSpace(body.Search)
			}
			if strings.TrimSpace(body.Province) != "" {
				province = strings.TrimSpace(body.Province)
			} else if strings.TrimSpace(body.Provider) != "" {
				province = strings.TrimSpace(body.Provider)
			}
			if strings.TrimSpace(body.StartDate) != "" {
				startDateStr = strings.TrimSpace(body.StartDate)
			} else if strings.TrimSpace(body.StartDate2) != "" {
				startDateStr = strings.TrimSpace(body.StartDate2)
			}
			if strings.TrimSpace(body.EndDate) != "" {
				endDateStr = strings.TrimSpace(body.EndDate)
			} else if strings.TrimSpace(body.EndDate2) != "" {
				endDateStr = strings.TrimSpace(body.EndDate2)
			}
		}
	}

	filter := domainShopee.OrderFilter{
		Search:   search,
		Province: province,
		Page:     page,
		Limit:    limit,
	}

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			filter.StartDate = &t
		}
	}

	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			end := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filter.EndDate = &end
		}
	}

	orders, total, err := h.orderUsecase.GetOrders(c.Context(), filter)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.List(c, orders, page, limit, total)
}

// GetOrderByID returns single order details
func (h *ShopeeHandler) GetOrderByID(c *fiber.Ctx) error {
	id := c.Params("id")
	ord, err := h.orderUsecase.GetOrderByID(c.Context(), id)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	if ord == nil {
		return response.NotFound(c, "Shopee order not found")
	}
	return response.OK(c, ord)
}

type updateSKUReq struct {
	SKU string `json:"sku"`
}

// UpdateItemSKU updates item SKU with confirmation
func (h *ShopeeHandler) UpdateItemSKU(c *fiber.Ctx) error {
	idParam := c.Params("id")
	itemID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid item ID")
	}

	var req updateSKUReq
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request payload")
	}

	if err := h.orderUsecase.UpdateItemSKU(c.Context(), uint(itemID), req.SKU); err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.OK(c, map[string]string{"message": "SKU updated successfully"})
}

// DeleteOrder deletes an order
func (h *ShopeeHandler) DeleteOrder(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.orderUsecase.DeleteOrder(c.Context(), id); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.OK(c, map[string]string{"message": "Order deleted successfully"})
}

// PreviewIncome parses income settlement file and returns preview
func (h *ShopeeHandler) PreviewIncome(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "Please upload a file (.xlsx, .xls, .csv)")
	}

	f, err := file.Open()
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to open file: %v", err))
	}
	defer f.Close()

	res, err := h.incomeUsecase.PreviewIncomeFile(c.Context(), f, file.Filename)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.OK(c, res)
}

// ImportIncome parses and commits income file
func (h *ShopeeHandler) ImportIncome(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.BadRequest(c, "Please upload a file (.xlsx, .xls, .csv)")
	}

	f, err := file.Open()
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to open file: %v", err))
	}
	defer f.Close()

	res, err := h.incomeUsecase.ImportIncomeFile(c.Context(), f, file.Filename)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.OK(c, res)
}

// GetIncomes returns paginated income records
func (h *ShopeeHandler) GetIncomes(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := strings.TrimSpace(c.Query("search"))
	status := strings.TrimSpace(c.Query("status"))
	startDateStr := strings.TrimSpace(c.Query("start_date"))
	endDateStr := strings.TrimSpace(c.Query("end_date"))

	if c.Method() == fiber.MethodPost {
		var body struct {
			Page       int    `json:"page"`
			Limit      int    `json:"limit"`
			Search     string `json:"search"`
			Status     string `json:"status"`
			StartDate  string `json:"start_date"`
			StartDate2 string `json:"startDate"`
			EndDate    string `json:"end_date"`
			EndDate2   string `json:"endDate"`
		}
		if err := c.BodyParser(&body); err == nil {
			if body.Page > 0 {
				page = body.Page
			}
			if body.Limit > 0 {
				limit = body.Limit
			}
			if strings.TrimSpace(body.Search) != "" {
				search = strings.TrimSpace(body.Search)
			}
			if strings.TrimSpace(body.Status) != "" {
				status = strings.TrimSpace(body.Status)
			}
			if strings.TrimSpace(body.StartDate) != "" {
				startDateStr = strings.TrimSpace(body.StartDate)
			} else if strings.TrimSpace(body.StartDate2) != "" {
				startDateStr = strings.TrimSpace(body.StartDate2)
			}
			if strings.TrimSpace(body.EndDate) != "" {
				endDateStr = strings.TrimSpace(body.EndDate)
			} else if strings.TrimSpace(body.EndDate2) != "" {
				endDateStr = strings.TrimSpace(body.EndDate2)
			}
		}
	}

	filter := domainShopee.IncomeFilter{
		Search: search,
		Status: status,
		Page:   page,
		Limit:  limit,
	}

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			filter.StartDate = &t
		}
	}
	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			end := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filter.EndDate = &end
		}
	}

	items, total, err := h.incomeUsecase.GetIncomes(c.Context(), filter)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.List(c, items, page, limit, total)
}

// DeleteIncome deletes an income record
func (h *ShopeeHandler) DeleteIncome(c *fiber.Ctx) error {
	idParam := c.Params("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid income ID")
	}

	if err := h.incomeUsecase.DeleteIncome(c.Context(), uint(id)); err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.OK(c, map[string]string{"message": "Income record deleted successfully"})
}

func parseMonthParam(c *fiber.Ctx) (int, int, error) {
	monthStr := c.Query("month")
	if monthStr == "" {
		now := time.Now()
		return now.Year(), int(now.Month()), nil
	}

	parts := strings.Split(monthStr, "-")
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid month format, expected YYYY-MM")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, errors.New("invalid year in month parameter")
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return 0, 0, errors.New("invalid month number (1-12)")
	}

	return year, month, nil
}

// GetMatching returns monthly matching calculation
func (h *ShopeeHandler) GetMatching(c *fiber.Ctx) error {
	year, month, err := parseMonthParam(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	rows, summary, err := h.matchingUsecase.GetMonthlyMatching(c.Context(), year, month)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, fiber.Map{
		"summary": summary,
		"items":   rows,
	})
}

// ExportMatchingCSV returns RFC 4180 CSV export
func (h *ShopeeHandler) ExportMatchingCSV(c *fiber.Ctx) error {
	year, month, err := parseMonthParam(c)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	data, filename, err := h.matchingUsecase.ExportMonthlyMatchingCSV(c.Context(), year, month)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	return c.Send(data)
}

// GetDashboard returns financial analytics and top SKUs
func (h *ShopeeHandler) GetDashboard(c *fiber.Ctx) error {
	now := time.Now()
	fromMonthStr := c.Query("from", fmt.Sprintf("%04d-01", now.Year()))
	toMonthStr := c.Query("to", fmt.Sprintf("%04d-%02d", now.Year(), now.Month()))

	fromParts := strings.Split(fromMonthStr, "-")
	toParts := strings.Split(toMonthStr, "-")

	fromY, _ := strconv.Atoi(fromParts[0])
	fromM, _ := strconv.Atoi(fromParts[1])
	toY, _ := strconv.Atoi(toParts[0])
	toM, _ := strconv.Atoi(toParts[1])

	if fromY == 0 || fromM == 0 {
		fromY, fromM = now.Year(), 1
	}
	if toY == 0 || toM == 0 {
		toY, toM = now.Year(), int(now.Month())
	}

	res, err := h.dashboardUsecase.GetDashboardAnalytics(c.Context(), fromY, fromM, toY, toM)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.OK(c, res)
}
