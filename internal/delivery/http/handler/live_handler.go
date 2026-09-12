package handler

import (
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	domainLive "chawy-erp-api/internal/domain/live"
	usecaseLive "chawy-erp-api/internal/usecase/live"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type LiveHandler struct {
	usecase usecaseLive.Usecase
}

func NewLiveHandler(usecase usecaseLive.Usecase) *LiveHandler {
	return &LiveHandler{usecase: usecase}
}

// CreateSession godoc
// POST /api/v1/live/sessions
func (h *LiveHandler) CreateSession(c *fiber.Ctx) error {
	var req dto.CreateLiveSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		userName = "Staff"
	}

	result, err := h.usecase.CreateSession(c.Context(), usecaseLive.CreateSessionInput{
		StaffID:          req.StaffID,
		LiveDate:         req.LiveDate,
		Platform:         req.Platform,
		TiktokAccount:    req.TiktokAccount,
		StartDatetime:    req.StartDatetime,
		EndDatetime:      req.EndDatetime,
		BreakMinutes:     req.BreakMinutes,
		RevenueGenerated: req.RevenueGenerated,
		HasClip:          req.HasClip,
		ClipLink:         req.ClipLink,
		LiveSummaryImage: req.LiveSummaryImage,
		HostNotes:        req.HostNotes,
		CreatedBy:        userName,
	})
	if err != nil {
		return err
	}

	return response.Created(c, result, "Live session recorded successfully")
}

// UpdateSession godoc
// PUT /api/v1/live/sessions/:id
func (h *LiveHandler) UpdateSession(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid session ID")
	}

	var req dto.UpdateLiveSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		userName = "Staff"
	}

	result, err := h.usecase.UpdateSession(c.Context(), uint(id), usecaseLive.UpdateSessionInput{
		Platform:         req.Platform,
		TiktokAccount:    req.TiktokAccount,
		StartDatetime:    req.StartDatetime,
		EndDatetime:      req.EndDatetime,
		BreakMinutes:     req.BreakMinutes,
		RevenueGenerated: req.RevenueGenerated,
		HasClip:          req.HasClip,
		ClipLink:         req.ClipLink,
		LiveSummaryImage: req.LiveSummaryImage,
		HostNotes:        req.HostNotes,
		UpdatedBy:        userName,
	})
	if err != nil {
		return err
	}

	return response.OK(c, result, "Live session updated successfully")
}

// GetSessionByID godoc
// GET /api/v1/live/sessions/:id
func (h *LiveHandler) GetSessionByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid session ID")
	}

	result, err := h.usecase.GetSessionByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, result)
}

// ListSessions godoc
// GET /api/v1/live/sessions
func (h *LiveHandler) ListSessions(c *fiber.Ctx) error {
	month := c.Query("month")
	status := c.Query("status")
	platform := c.Query("platform")
	staffIDStr := c.Query("staff_id")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	var staffID uint
	if staffIDStr != "" {
		val, _ := strconv.ParseUint(staffIDStr, 10, 32)
		staffID = uint(val)
	}

	sessions, total, err := h.usecase.ListSessions(c.Context(), domainLive.SessionFilter{
		Month:    month,
		Status:   status,
		Platform: platform,
		StaffID:  staffID,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, sessions, page, limit, total)
}

// ApproveSession godoc
// POST /api/v1/live/sessions/:id/approve
func (h *LiveHandler) ApproveSession(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid session ID")
	}

	userID, _ := c.Locals("user_id").(uint)
	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		userName = "Owner"
	}

	result, err := h.usecase.ApproveSession(c.Context(), uint(id), userID, userName)
	if err != nil {
		return err
	}

	return response.OK(c, result, "Live session approved successfully")
}

// RejectSession godoc
// POST /api/v1/live/sessions/:id/reject
func (h *LiveHandler) RejectSession(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid session ID")
	}

	var req dto.RejectLiveSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		userName = "Owner"
	}

	result, err := h.usecase.RejectSession(c.Context(), uint(id), req.Reason, userName)
	if err != nil {
		return err
	}

	return response.OK(c, result, "Live session rejected")
}

// GetPayrollSummary godoc
// GET /api/v1/live/payroll
func (h *LiveHandler) GetPayrollSummary(c *fiber.Ctx) error {
	month := c.Query("month")
	rounding := domainLive.RoundingPolicy(c.Query("rounding", string(domainLive.RoundingQuarterUp)))

	result, err := h.usecase.GetPayrollSummary(c.Context(), month, rounding)
	if err != nil {
		return err
	}

	return response.OK(c, result)
}

// ListContentItems godoc
// GET /api/v1/live/content
func (h *LiveHandler) ListContentItems(c *fiber.Ctx) error {
	kind := c.Query("kind")
	platform := c.Query("platform")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	items, total, err := h.usecase.ListContentItems(c.Context(), domainLive.ContentFilter{
		Kind:     kind,
		Platform: platform,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, items, page, limit, total)
}

// CreateContentItem godoc
// POST /api/v1/live/content
func (h *LiveHandler) CreateContentItem(c *fiber.Ctx) error {
	var req dto.CreateContentItemRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	userName, _ := c.Locals("user_name").(string)
	if userName == "" {
		userName = "Staff"
	}

	result, err := h.usecase.CreateContentItem(c.Context(), usecaseLive.CreateContentInput{
		Kind:          req.Kind,
		Title:         req.Title,
		Platform:      req.Platform,
		ScheduledFor:  req.ScheduledFor,
		PublishedAt:   req.PublishedAt,
		HostID:        req.HostID,
		HostName:      req.HostName,
		Reach:         req.Reach,
		EngagementPct: req.EngagementPct,
		Notes:         req.Notes,
		CreatedBy:     userName,
	})
	if err != nil {
		return err
	}

	return response.Created(c, result, "Content item created successfully")
}

// UpdateContentItem godoc
// PUT /api/v1/live/content/:id
func (h *LiveHandler) UpdateContentItem(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid content item ID")
	}

	var req dto.UpdateContentItemRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	result, err := h.usecase.UpdateContentItem(c.Context(), uint(id), usecaseLive.UpdateContentInput{
		Title:         req.Title,
		Platform:      req.Platform,
		ScheduledFor:  req.ScheduledFor,
		PublishedAt:   req.PublishedAt,
		HostID:        req.HostID,
		HostName:      req.HostName,
		Reach:         req.Reach,
		EngagementPct: req.EngagementPct,
		Notes:         req.Notes,
	})
	if err != nil {
		return err
	}

	return response.OK(c, result, "Content item updated successfully")
}

// DeleteContentItem godoc
// DELETE /api/v1/live/content/:id
func (h *LiveHandler) DeleteContentItem(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid content item ID")
	}

	if err := h.usecase.DeleteContentItem(c.Context(), uint(id)); err != nil {
		return err
	}

	return response.OK(c, nil, "Content item deleted successfully")
}
