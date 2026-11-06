package live

import (
	"time"

	domainAuth "chawy-erp-api/internal/domain/auth"
)

type LiveSessionStatus string

const (
	StatusPending  LiveSessionStatus = "PENDING"
	StatusApproved LiveSessionStatus = "APPROVED"
	StatusRejected LiveSessionStatus = "REJECTED"
)

type LivePlatform string

const (
	PlatformTikTok LivePlatform = "TIKTOK"
	PlatformShopee LivePlatform = "SHOPEE"
	PlatformLazada LivePlatform = "LAZADA"
)

// LiveSession represents a single live streaming session record.
type LiveSession struct {
	ID               uint              `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionNo        string            `gorm:"size:50;uniqueIndex;not null" json:"session_no"`
	StaffID          uint              `gorm:"index;not null" json:"staff_id"`
	Staff            *domainAuth.User  `gorm:"foreignKey:StaffID" json:"staff,omitempty"`
	LiveDate         string            `gorm:"size:20;index;not null" json:"live_date"` // YYYY-MM-DD
	Platform         LivePlatform      `gorm:"size:50;not null;default:'TIKTOK'" json:"platform"`
	TiktokAccount    string            `gorm:"size:100" json:"tiktok_account"`
	StartDatetime    time.Time         `gorm:"index;not null" json:"start_datetime"`
	EndDatetime      time.Time         `gorm:"index;not null" json:"end_datetime"`
	BreakMinutes     int               `gorm:"default:0" json:"break_minutes"`
	RevenueGenerated float64           `gorm:"type:numeric(15,2);default:0" json:"revenue_generated"`
	HasClip          bool              `gorm:"default:false" json:"has_clip"`
	ClipLink         string            `gorm:"size:500" json:"clip_link"`
	LiveSummaryImage string            `gorm:"size:500" json:"live_summary_image"`
	HostNotes        string            `gorm:"type:text" json:"host_notes"`
	Status           LiveSessionStatus `gorm:"size:50;index;not null;default:'PENDING'" json:"status"`
	ApprovedBy       *uint             `gorm:"index" json:"approved_by,omitempty"`
	RejectionReason  string            `gorm:"size:500" json:"rejection_reason"`
	CreatedBy        string            `gorm:"size:100" json:"created_by"`
	UpdatedBy        string            `gorm:"size:100" json:"updated_by"`
	AuditTrail       string            `gorm:"type:text" json:"audit_trail"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type ContentKind string

const (
	ContentKindScheduled ContentKind = "SCHEDULED"
	ContentKindPublished ContentKind = "PUBLISHED"
)

// ContentItem represents scheduled planning or published content performance.
type ContentItem struct {
	ID             uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	Kind           ContentKind      `gorm:"size:50;index;not null" json:"kind"`
	Title          string           `gorm:"size:255;not null" json:"title"`
	Platform       string           `gorm:"size:50;not null" json:"platform"`
	ScheduledFor   *time.Time       `gorm:"index" json:"scheduled_for,omitempty"`
	PublishedAt    *time.Time       `gorm:"index" json:"published_at,omitempty"`
	HostID         *uint            `gorm:"index" json:"host_id,omitempty"`
	Host           *domainAuth.User `gorm:"foreignKey:HostID" json:"host,omitempty"`
	HostName       string           `gorm:"size:100" json:"host_name"`
	Reach          int64            `gorm:"default:0" json:"reach"`
	EngagementPct  float64          `gorm:"type:numeric(5,2);default:0" json:"engagement_pct"`
	Notes          string           `gorm:"type:text" json:"notes"`
	CreatedBy      string           `gorm:"size:100" json:"created_by"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// RoundingPolicy specifies how net minutes are rounded before pay calculation.
type RoundingPolicy string

const (
	RoundingActual    RoundingPolicy = "actual"
	RoundingQuarterUp RoundingPolicy = "quarter_up" // UP 15 mins
	RoundingUp10      RoundingPolicy = "up10"
	RoundingUp30      RoundingPolicy = "up30"
)

// PayrollRow represents calculated payroll for an individual staff in a given month.
type PayrollRow struct {
	StaffID               uint    `json:"staff_id"`
	StaffName             string  `json:"staff_name"`
	ApprovedSessionsCount int     `json:"approved_sessions_count"`
	TotalNetMinutes       int     `json:"total_net_minutes"`
	RoundedMinutes        int     `json:"rounded_minutes"`
	DecimalHours          float64 `json:"decimal_hours"`
	HourlyRate            int     `json:"hourly_rate"`
	BasePay               float64 `json:"base_pay"`
	ClipBonusCount        int     `json:"clip_bonus_count"`
	ClipBonusRate         int     `json:"clip_bonus_rate"`
	ClipBonusPay          float64 `json:"clip_bonus_pay"`
	TotalPay              float64 `json:"total_pay"`
}

// PayrollSummary aggregates monthly live payroll.
type PayrollSummary struct {
	Month          string       `json:"month"` // YYYY-MM
	RoundingPolicy string       `json:"rounding_policy"`
	TotalHours     float64      `json:"total_hours"`
	TotalPay       float64      `json:"total_pay"`
	TotalClips     int          `json:"total_clips"`
	StaffPayroll   []PayrollRow `json:"staff_payroll"`
}

// SessionFilter provides query parameters for listing live sessions.
type SessionFilter struct {
	Month    string // YYYY-MM
	Status   string
	StaffID  uint
	Platform string
	Page     int
	Limit    int
}

// ContentFilter provides query parameters for listing content items.
type ContentFilter struct {
	Kind     string
	Platform string
	Page     int
	Limit    int
}
