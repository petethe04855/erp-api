package dto

import (
	"time"

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
