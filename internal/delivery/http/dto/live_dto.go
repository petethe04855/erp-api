package dto

import (
	"time"

	domainAuth "chawy-erp-api/internal/domain/auth"
	domainLive "chawy-erp-api/internal/domain/live"
)

type CreateLiveSessionRequest struct {
	StaffID          uint                    `json:"staff_id"`
	LiveDate         string                  `json:"live_date"`
	Platform         domainLive.LivePlatform `json:"platform"`
	TiktokAccount    string                  `json:"tiktok_account"`
	StartDatetime    time.Time               `json:"start_datetime"`
	EndDatetime      time.Time               `json:"end_datetime"`
	BreakMinutes     int                     `json:"break_minutes"`
	RevenueGenerated float64                 `json:"revenue_generated"`
	HasClip          bool                    `json:"has_clip"`
	ClipLink         string                  `json:"clip_link"`
	LiveSummaryImage string                  `json:"live_summary_image"`
	HostNotes        string                  `json:"host_notes"`
}

type UpdateLiveSessionRequest struct {
	Platform         domainLive.LivePlatform `json:"platform"`
	TiktokAccount    string                  `json:"tiktok_account"`
	StartDatetime    time.Time               `json:"start_datetime"`
	EndDatetime      time.Time               `json:"end_datetime"`
	BreakMinutes     int                     `json:"break_minutes"`
	RevenueGenerated float64                 `json:"revenue_generated"`
	HasClip          bool                    `json:"has_clip"`
	ClipLink         string                  `json:"clip_link"`
	LiveSummaryImage string                  `json:"live_summary_image"`
	HostNotes        string                  `json:"host_notes"`
}

type RejectLiveSessionRequest struct {
	Reason string `json:"reason"`
}

type CreateContentItemRequest struct {
	Kind          domainLive.ContentKind `json:"kind"`
	Title         string                 `json:"title"`
	Platform      string                 `json:"platform"`
	ScheduledFor  *time.Time             `json:"scheduled_for,omitempty"`
	PublishedAt   *time.Time             `json:"published_at,omitempty"`
	HostID        *uint                  `json:"host_id,omitempty"`
	HostName      string                 `json:"host_name"`
	Reach         int64                  `json:"reach"`
	EngagementPct float64                `json:"engagement_pct"`
	Notes         string                 `json:"notes"`
}

type UpdateContentItemRequest struct {
	Title         string     `json:"title"`
	Platform      string     `json:"platform"`
	ScheduledFor  *time.Time `json:"scheduled_for,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	HostID        *uint      `json:"host_id,omitempty"`
	HostName      string     `json:"host_name"`
	Reach         int64      `json:"reach"`
	EngagementPct float64    `json:"engagement_pct"`
	Notes         string     `json:"notes"`
}

// LiveUserRefResponse is the nested staff/host reference embedded in live
// payloads. It mirrors the previous domain user output field-for-field.
type LiveUserRefResponse struct {
	ID          uint       `json:"id"`
	Email       string     `json:"email"`
	Firstname   string     `json:"firstname"`
	Lastname    string     `json:"lastname"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	IsActive    bool       `json:"isActive"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// LiveSessionResponse is the API contract for a live streaming session.
type LiveSessionResponse struct {
	ID               uint                       `json:"id"`
	SessionNo        string                     `json:"session_no"`
	StaffID          uint                       `json:"staff_id"`
	Staff            *LiveUserRefResponse       `json:"staff,omitempty"`
	LiveDate         string                     `json:"live_date"`
	Platform         domainLive.LivePlatform    `json:"platform"`
	TiktokAccount    string                     `json:"tiktok_account"`
	StartDatetime    time.Time                  `json:"start_datetime"`
	EndDatetime      time.Time                  `json:"end_datetime"`
	BreakMinutes     int                        `json:"break_minutes"`
	RevenueGenerated float64                    `json:"revenue_generated"`
	HasClip          bool                       `json:"has_clip"`
	ClipLink         string                     `json:"clip_link"`
	LiveSummaryImage string                     `json:"live_summary_image"`
	HostNotes        string                     `json:"host_notes"`
	Status           domainLive.LiveSessionStatus `json:"status"`
	ApprovedBy       *uint                      `json:"approved_by,omitempty"`
	RejectionReason  string                     `json:"rejection_reason"`
	CreatedBy        string                     `json:"created_by"`
	UpdatedBy        string                     `json:"updated_by"`
	AuditTrail       string                     `json:"audit_trail"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

// NewLiveUserRefResponse maps the optional nested staff/host user reference.
func NewLiveUserRefResponse(u *domainAuth.User) *LiveUserRefResponse {
	if u == nil {
		return nil
	}
	return &LiveUserRefResponse{
		ID:          u.ID,
		Email:       u.Email,
		Firstname:   u.Firstname,
		Lastname:    u.Lastname,
		Name:        u.Name,
		Role:        u.Role,
		IsActive:    u.IsActive,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

// NewLiveSessionResponse maps a domain live session to the API contract.
func NewLiveSessionResponse(s *domainLive.LiveSession) LiveSessionResponse {
	return LiveSessionResponse{
		ID:               s.ID,
		SessionNo:        s.SessionNo,
		StaffID:          s.StaffID,
		Staff:            NewLiveUserRefResponse(s.Staff),
		LiveDate:         s.LiveDate,
		Platform:         s.Platform,
		TiktokAccount:    s.TiktokAccount,
		StartDatetime:    s.StartDatetime,
		EndDatetime:      s.EndDatetime,
		BreakMinutes:     s.BreakMinutes,
		RevenueGenerated: s.RevenueGenerated,
		HasClip:          s.HasClip,
		ClipLink:         s.ClipLink,
		LiveSummaryImage: s.LiveSummaryImage,
		HostNotes:        s.HostNotes,
		Status:           s.Status,
		ApprovedBy:       s.ApprovedBy,
		RejectionReason:  s.RejectionReason,
		CreatedBy:        s.CreatedBy,
		UpdatedBy:        s.UpdatedBy,
		AuditTrail:       s.AuditTrail,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

// NewLiveSessionResponses maps a slice of domain live sessions.
func NewLiveSessionResponses(items []domainLive.LiveSession) []LiveSessionResponse {
	out := make([]LiveSessionResponse, len(items))
	for i := range items {
		out[i] = NewLiveSessionResponse(&items[i])
	}
	return out
}

// ContentItemResponse is the API contract for scheduled/published content.
type ContentItemResponse struct {
	ID            uint                 `json:"id"`
	Kind          domainLive.ContentKind `json:"kind"`
	Title         string               `json:"title"`
	Platform      string               `json:"platform"`
	ScheduledFor  *time.Time           `json:"scheduled_for,omitempty"`
	PublishedAt   *time.Time           `json:"published_at,omitempty"`
	HostID        *uint                `json:"host_id,omitempty"`
	Host          *LiveUserRefResponse `json:"host,omitempty"`
	HostName      string               `json:"host_name"`
	Reach         int64                `json:"reach"`
	EngagementPct float64              `json:"engagement_pct"`
	Notes         string               `json:"notes"`
	CreatedBy     string               `json:"created_by"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// NewContentItemResponse maps a domain content item to the API contract.
func NewContentItemResponse(it *domainLive.ContentItem) ContentItemResponse {
	return ContentItemResponse{
		ID:            it.ID,
		Kind:          it.Kind,
		Title:         it.Title,
		Platform:      it.Platform,
		ScheduledFor:  it.ScheduledFor,
		PublishedAt:   it.PublishedAt,
		HostID:        it.HostID,
		Host:          NewLiveUserRefResponse(it.Host),
		HostName:      it.HostName,
		Reach:         it.Reach,
		EngagementPct: it.EngagementPct,
		Notes:         it.Notes,
		CreatedBy:     it.CreatedBy,
		CreatedAt:     it.CreatedAt,
		UpdatedAt:     it.UpdatedAt,
	}
}

// NewContentItemResponses maps a slice of domain content items.
func NewContentItemResponses(items []domainLive.ContentItem) []ContentItemResponse {
	out := make([]ContentItemResponse, len(items))
	for i := range items {
		out[i] = NewContentItemResponse(&items[i])
	}
	return out
}
