package handler

import (
	"strconv"
	"time"

	"chawy-erp-api/internal/delivery/http/dto"
	domainReturn "chawy-erp-api/internal/domain/salesreturn"
	usecaseReturn "chawy-erp-api/internal/usecase/salesreturn"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type SalesReturnHandler struct {
	uc usecaseReturn.Usecase
}

func NewSalesReturnHandler(uc usecaseReturn.Usecase) *SalesReturnHandler {
	return &SalesReturnHandler{uc: uc}
}

func (h *SalesReturnHandler) List(c *fiber.Ctx) error {
	var bodyReq struct {
		Page        int    `json:"page"`
		Limit       int    `json:"limit"`
		WarehouseID uint   `json:"warehouse_id"`
		Search      string `json:"search"`
		Status      string `json:"status"`
		ReturnType  string `json:"return_type"`
		Channel     string `json:"channel"`
		StartDate   string `json:"start_date"`
		EndDate     string `json:"end_date"`
	}
	if c.Method() == fiber.MethodPost {
		_ = c.BodyParser(&bodyReq)
	}

	page := bodyReq.Page
	if page < 1 {
		page, _ = strconv.Atoi(c.Query("page", "1"))
	}
	if page < 1 {
		page = 1
	}

	limit := bodyReq.Limit
	if limit <= 0 {
		limit, _ = strconv.Atoi(c.Query("limit", "20"))
	}

	whID := bodyReq.WarehouseID
	if whID == 0 {
		w, _ := strconv.Atoi(c.Query("warehouse_id", "0"))
		whID = uint(w)
	}

	search := bodyReq.Search
	if search == "" {
		search = c.Query("search")
	}

	status := bodyReq.Status
	if status == "" {
		status = c.Query("status")
	}

	returnType := bodyReq.ReturnType
	if returnType == "" {
		returnType = c.Query("return_type")
	}

	channel := bodyReq.Channel
	if channel == "" {
		channel = c.Query("channel")
	}

	startDate := bodyReq.StartDate
	if startDate == "" {
		startDate = c.Query("start_date")
	}

	endDate := bodyReq.EndDate
	if endDate == "" {
		endDate = c.Query("end_date")
	}

	query := domainReturn.Query{
		Search:      search,
		Status:      status,
		ReturnType:  returnType,
		Channel:     channel,
		WarehouseID: whID,
		StartDate:   startDate,
		EndDate:     endDate,
		Page:        page,
		Limit:       limit,
	}

	items, total, err := h.uc.List(c.UserContext(), query)
	if err != nil {
		return err
	}

	return response.List(c, items, page, limit, total)
}

func (h *SalesReturnHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid return ID")
	}

	item, err := h.uc.GetByID(c.UserContext(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, item)
}

func (h *SalesReturnHandler) GetOrderReturnable(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	items, err := h.uc.GetOrderReturnable(c.UserContext(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, items)
}

func (h *SalesReturnHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateReturnRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	username, _ := c.Locals("username").(string)
	if username == "" {
		username = "System"
	}

	var retDate *time.Time
	if req.ReturnDate != "" {
		if t, err := time.Parse("2006-01-02", req.ReturnDate); err == nil {
			retDate = &t
		}
	}

	var lines []usecaseReturn.CreateReturnLineInput
	for _, l := range req.Lines {
		lines = append(lines, usecaseReturn.CreateReturnLineInput{
			SKU:            l.SKU,
			Quantity:       l.Quantity,
			Condition:      l.Condition,
			Restock:        l.Restock,
			ReasonCode:     l.ReasonCode,
			EvidenceImages: l.EvidenceImages,
			LotRef:         l.LotRef,
		})
	}

	input := usecaseReturn.CreateReturnInput{
		ReturnType:  req.ReturnType,
		OrderID:     req.OrderID,
		WarehouseID: req.WarehouseID,
		ReturnDate:  retDate,
		Reason:      req.Reason,
		Note:        req.Note,
		CreatedBy:   username,
		Lines:       lines,
	}

	item, err := h.uc.Create(c.UserContext(), input)
	if err != nil {
		return err
	}

	return response.Created(c, item)
}

func (h *SalesReturnHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid return ID")
	}

	var req dto.UpdateReturnRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var retDate *time.Time
	if req.ReturnDate != "" {
		if t, err := time.Parse("2006-01-02", req.ReturnDate); err == nil {
			retDate = &t
		}
	}

	var lines []usecaseReturn.CreateReturnLineInput
	for _, l := range req.Lines {
		lines = append(lines, usecaseReturn.CreateReturnLineInput{
			SKU:            l.SKU,
			Quantity:       l.Quantity,
			Condition:      l.Condition,
			Restock:        l.Restock,
			ReasonCode:     l.ReasonCode,
			EvidenceImages: l.EvidenceImages,
			LotRef:         l.LotRef,
		})
	}

	input := usecaseReturn.UpdateReturnInput{
		WarehouseID: req.WarehouseID,
		ReturnDate:  retDate,
		Reason:      req.Reason,
		Note:        req.Note,
		Lines:       lines,
	}

	item, err := h.uc.Update(c.UserContext(), uint(id), input)
	if err != nil {
		return err
	}

	return response.OK(c, item)
}

func (h *SalesReturnHandler) Submit(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid return ID")
	}

	item, err := h.uc.Submit(c.UserContext(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, item)
}

func (h *SalesReturnHandler) Approve(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid return ID")
	}

	username, _ := c.Locals("username").(string)
	item, err := h.uc.Approve(c.UserContext(), uint(id), username)
	if err != nil {
		return err
	}

	return response.OK(c, item)
}

func (h *SalesReturnHandler) Reject(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid return ID")
	}

	var req dto.ReasonRequest
	_ = c.BodyParser(&req)

	username, _ := c.Locals("username").(string)
	item, err := h.uc.Reject(c.UserContext(), uint(id), req.Reason, username)
	if err != nil {
		return err
	}

	return response.OK(c, item)
}

func (h *SalesReturnHandler) Cancel(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid return ID")
	}

	var req dto.ReasonRequest
	_ = c.BodyParser(&req)

	username, _ := c.Locals("username").(string)
	item, err := h.uc.Cancel(c.UserContext(), uint(id), req.Reason, username)
	if err != nil {
		return err
	}

	return response.OK(c, item)
}

func (h *SalesReturnHandler) Complete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid return ID")
	}

	var req dto.CompleteReturnRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	username, _ := c.Locals("username").(string)
	var lines []usecaseReturn.CompleteReturnLineInput
	for _, l := range req.Lines {
		lines = append(lines, usecaseReturn.CompleteReturnLineInput{
			LineID:            l.LineID,
			Condition:         l.Condition,
			Restock:           l.Restock,
			AddEvidenceImages: l.AddEvidenceImages,
		})
	}

	input := usecaseReturn.CompleteReturnInput{
		Lines:       lines,
		CompletedBy: username,
	}

	item, err := h.uc.Complete(c.UserContext(), uint(id), input)
	if err != nil {
		return err
	}

	return response.OK(c, item)
}
