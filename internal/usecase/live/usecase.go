package live

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	domainAuth "chawy-erp-api/internal/domain/auth"
	domainLive "chawy-erp-api/internal/domain/live"
	domainSettings "chawy-erp-api/internal/domain/settings"
	"chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"

	"gorm.io/gorm"
)

type CreateSessionInput struct {
	StaffID          uint                    `json:"staff_id"`
	LiveDate         string                  `json:"live_date"` // YYYY-MM-DD
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
	CreatedBy        string                  `json:"created_by"`
}

type UpdateSessionInput struct {
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
	UpdatedBy        string                  `json:"updated_by"`
}

type AuditEvent struct {
	Action string `json:"action"`
	By     string `json:"by"`
	At     string `json:"at"`
	Note   string `json:"note,omitempty"`
}

type CreateContentInput struct {
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
	CreatedBy     string                 `json:"created_by"`
}

type UpdateContentInput struct {
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

type Usecase interface {
	CreateSession(ctx context.Context, input CreateSessionInput) (*domainLive.LiveSession, error)
	UpdateSession(ctx context.Context, id uint, input UpdateSessionInput) (*domainLive.LiveSession, error)
	GetSessionByID(ctx context.Context, id uint) (*domainLive.LiveSession, error)
	ListSessions(ctx context.Context, filter domainLive.SessionFilter) ([]domainLive.LiveSession, int64, error)
	ApproveSession(ctx context.Context, id uint, approvedByID uint, approverName string) (*domainLive.LiveSession, error)
	RejectSession(ctx context.Context, id uint, reason string, rejectedByName string) (*domainLive.LiveSession, error)
	GetPayrollSummary(ctx context.Context, month string, policy domainLive.RoundingPolicy) (*domainLive.PayrollSummary, error)

	CreateContentItem(ctx context.Context, input CreateContentInput) (*domainLive.ContentItem, error)
	UpdateContentItem(ctx context.Context, id uint, input UpdateContentInput) (*domainLive.ContentItem, error)
	DeleteContentItem(ctx context.Context, id uint) error
	GetContentItemByID(ctx context.Context, id uint) (*domainLive.ContentItem, error)
	ListContentItems(ctx context.Context, filter domainLive.ContentFilter) ([]domainLive.ContentItem, int64, error)
}

type liveUsecase struct {
	db           *gorm.DB
	liveRepo     domainLive.Repository
	settingsRepo domainSettings.Repository
	authRepo     domainAuth.Repository
	txMgr        database.TxManager
}

func NewLiveUsecase(
	db *gorm.DB,
	liveRepo domainLive.Repository,
	settingsRepo domainSettings.Repository,
	authRepo domainAuth.Repository,
) Usecase {
	var txMgr database.TxManager
	if db != nil {
		txMgr = database.NewTxManager(db)
	}
	return &liveUsecase{
		db:           db,
		liveRepo:     liveRepo,
		settingsRepo: settingsRepo,
		authRepo:     authRepo,
		txMgr:        txMgr,
	}
}

// CalculateNetMinutes calculates net live duration in minutes (cross-midnight aware) subtracting break
func CalculateNetMinutes(start, end time.Time, breakMinutes int) (int, error) {
	if start.IsZero() || end.IsZero() {
		return 0, domainLive.ErrInvalidSessionTimes
	}

	// Cross-midnight adjustment if end is before start (e.g. 23:00 to 02:00 next day)
	if end.Before(start) {
		end = end.Add(24 * time.Hour)
	}

	diffMinutes := int(math.Round(end.Sub(start).Minutes()))
	if diffMinutes <= 0 {
		return 0, domainLive.ErrInvalidSessionTimes
	}

	if breakMinutes < 0 {
		breakMinutes = 0
	}
	if breakMinutes >= diffMinutes {
		return 0, fmt.Errorf("break time (%d mins) cannot exceed or equal total duration (%d mins)", breakMinutes, diffMinutes)
	}

	return diffMinutes - breakMinutes, nil
}

// ApplyRounding rounds net minutes based on the chosen RoundingPolicy
func ApplyRounding(minutes int, policy domainLive.RoundingPolicy) int {
	if minutes <= 0 {
		return 0
	}
	switch policy {
	case domainLive.RoundingQuarterUp: // Up 15 minutes
		return int(math.Ceil(float64(minutes)/15.0)) * 15
	case domainLive.RoundingUp10:
		return int(math.Ceil(float64(minutes)/10.0)) * 10
	case domainLive.RoundingUp30:
		return int(math.Ceil(float64(minutes)/30.0)) * 30
	case domainLive.RoundingActual:
		fallthrough
	default:
		return minutes
	}
}

func (u *liveUsecase) CreateSession(ctx context.Context, input CreateSessionInput) (*domainLive.LiveSession, error) {
	if input.StaffID == 0 {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", "Staff ID is required", 400)
	}
	if input.LiveDate == "" {
		input.LiveDate = input.StartDatetime.Format("2006-01-02")
	}

	// Cross-midnight check & normalize end
	end := input.EndDatetime
	if end.Before(input.StartDatetime) {
		end = end.Add(24 * time.Hour)
	}

	if _, err := CalculateNetMinutes(input.StartDatetime, end, input.BreakMinutes); err != nil {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", err.Error(), 400)
	}

	// Check overlap
	overlap, existing, err := u.liveRepo.CheckOverlap(ctx, input.StaffID, input.StartDatetime, end, 0)
	if err != nil {
		return nil, err
	}
	if overlap && existing != nil {
		return nil, appErrors.NewAppError("LIVE_OVERLAP", fmt.Sprintf("Session overlaps with %s (%s - %s)", existing.SessionNo, existing.StartDatetime.Format("15:04"), existing.EndDatetime.Format("15:04")), 409)
	}

	sessionNo, err := u.liveRepo.GetNextSessionNo(ctx, input.LiveDate)
	if err != nil {
		return nil, err
	}

	platform := input.Platform
	if platform == "" {
		platform = domainLive.PlatformTikTok
	}

	now := time.Now()
	audit := []AuditEvent{
		{
			Action: "Created",
			By:     input.CreatedBy,
			At:     now.Format(time.RFC3339),
			Note:   "Live session recorded",
		},
	}
	auditBytes, _ := json.Marshal(audit)

	session := &domainLive.LiveSession{
		SessionNo:        sessionNo,
		StaffID:          input.StaffID,
		LiveDate:         input.LiveDate,
		Platform:         platform,
		TiktokAccount:    input.TiktokAccount,
		StartDatetime:    input.StartDatetime,
		EndDatetime:      end,
		BreakMinutes:     input.BreakMinutes,
		RevenueGenerated: input.RevenueGenerated,
		HasClip:          input.HasClip,
		ClipLink:         input.ClipLink,
		LiveSummaryImage: input.LiveSummaryImage,
		HostNotes:        input.HostNotes,
		Status:           domainLive.StatusPending,
		CreatedBy:        input.CreatedBy,
		UpdatedBy:        input.CreatedBy,
		AuditTrail:       string(auditBytes),
	}

	if err := u.liveRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	return u.liveRepo.GetSessionByID(ctx, session.ID)
}

func (u *liveUsecase) UpdateSession(ctx context.Context, id uint, input UpdateSessionInput) (*domainLive.LiveSession, error) {
	session, err := u.liveRepo.GetSessionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if session.Status != domainLive.StatusPending {
		return nil, appErrors.NewAppError("LIVE_NOT_EDITABLE", "Only pending sessions can be edited", 400)
	}

	end := input.EndDatetime
	if end.Before(input.StartDatetime) {
		end = end.Add(24 * time.Hour)
	}

	if _, err := CalculateNetMinutes(input.StartDatetime, end, input.BreakMinutes); err != nil {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", err.Error(), 400)
	}

	// Check overlap excluding current
	overlap, existing, err := u.liveRepo.CheckOverlap(ctx, session.StaffID, input.StartDatetime, end, session.ID)
	if err != nil {
		return nil, err
	}
	if overlap && existing != nil {
		return nil, appErrors.NewAppError("LIVE_OVERLAP", fmt.Sprintf("Session overlaps with %s", existing.SessionNo), 409)
	}

	if input.Platform != "" {
		session.Platform = input.Platform
	}
	session.TiktokAccount = input.TiktokAccount
	session.StartDatetime = input.StartDatetime
	session.EndDatetime = end
	session.BreakMinutes = input.BreakMinutes
	session.RevenueGenerated = input.RevenueGenerated
	session.HasClip = input.HasClip
	session.ClipLink = input.ClipLink
	session.LiveSummaryImage = input.LiveSummaryImage
	session.HostNotes = input.HostNotes
	session.UpdatedBy = input.UpdatedBy

	// Append audit
	var audit []AuditEvent
	if len(session.AuditTrail) > 0 {
		_ = json.Unmarshal([]byte(session.AuditTrail), &audit)
	}
	audit = append(audit, AuditEvent{
		Action: "Updated",
		By:     input.UpdatedBy,
		At:     time.Now().Format(time.RFC3339),
		Note:   "Live session details edited",
	})
	auditBytes, _ := json.Marshal(audit)
	session.AuditTrail = string(auditBytes)

	if err := u.liveRepo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}

	return u.liveRepo.GetSessionByID(ctx, session.ID)
}

func (u *liveUsecase) GetSessionByID(ctx context.Context, id uint) (*domainLive.LiveSession, error) {
	return u.liveRepo.GetSessionByID(ctx, id)
}

func (u *liveUsecase) ListSessions(ctx context.Context, filter domainLive.SessionFilter) ([]domainLive.LiveSession, int64, error) {
	return u.liveRepo.ListSessions(ctx, filter)
}

func (u *liveUsecase) ApproveSession(ctx context.Context, id uint, approvedByID uint, approverName string) (*domainLive.LiveSession, error) {
	session, err := u.liveRepo.GetSessionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if session.Status == domainLive.StatusApproved {
		return session, nil
	}

	session.Status = domainLive.StatusApproved
	session.ApprovedBy = &approvedByID
	session.UpdatedBy = approverName

	var audit []AuditEvent
	if len(session.AuditTrail) > 0 {
		_ = json.Unmarshal([]byte(session.AuditTrail), &audit)
	}
	audit = append(audit, AuditEvent{
		Action: "Approved",
		By:     approverName,
		At:     time.Now().Format(time.RFC3339),
		Note:   "Live session verified and approved for payroll",
	})
	auditBytes, _ := json.Marshal(audit)
	session.AuditTrail = string(auditBytes)

	if err := u.liveRepo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}

	return u.liveRepo.GetSessionByID(ctx, session.ID)
}

func (u *liveUsecase) RejectSession(ctx context.Context, id uint, reason string, rejectedByName string) (*domainLive.LiveSession, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, appErrors.NewAppError("LIVE_REJECTION_REASON_REQUIRED", "Rejection reason is required", 400)
	}

	session, err := u.liveRepo.GetSessionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	session.Status = domainLive.StatusRejected
	session.RejectionReason = reason
	session.UpdatedBy = rejectedByName

	var audit []AuditEvent
	if len(session.AuditTrail) > 0 {
		_ = json.Unmarshal([]byte(session.AuditTrail), &audit)
	}
	audit = append(audit, AuditEvent{
		Action: "Rejected",
		By:     rejectedByName,
		At:     time.Now().Format(time.RFC3339),
		Note:   "Rejected: " + reason,
	})
	auditBytes, _ := json.Marshal(audit)
	session.AuditTrail = string(auditBytes)

	if err := u.liveRepo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}

	return u.liveRepo.GetSessionByID(ctx, session.ID)
}

func (u *liveUsecase) GetPayrollSummary(ctx context.Context, month string, policy domainLive.RoundingPolicy) (*domainLive.PayrollSummary, error) {
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if policy == "" {
		policy = domainLive.RoundingQuarterUp
	}

	// 1. Fetch settings for live payroll
	settings, err := u.settingsRepo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	payrollSettings := settings.LivePayroll
	defaultHourlyRate := payrollSettings.HourlyRate
	if defaultHourlyRate <= 0 {
		defaultHourlyRate = 120 // fallback standard rate
	}
	clipBonusRate := payrollSettings.ClipBonus

	// 2. Fetch approved sessions in month
	sessions, _, err := u.liveRepo.ListSessions(ctx, domainLive.SessionFilter{
		Month:  month,
		Status: string(domainLive.StatusApproved),
		Limit:  1000,
	})
	if err != nil {
		return nil, err
	}

	// 3. Group by StaffID
	type staffAgg struct {
		staffName     string
		sessionsCount int
		netMinutes    int
		clipCount     int
	}
	staffMap := make(map[uint]*staffAgg)

	for _, s := range sessions {
		agg, exists := staffMap[s.StaffID]
		if !exists {
			name := fmt.Sprintf("Staff #%d", s.StaffID)
			if s.Staff != nil && s.Staff.Name != "" {
				name = s.Staff.Name
			}
			agg = &staffAgg{
				staffName: name,
			}
			staffMap[s.StaffID] = agg
		}

		agg.sessionsCount++
		net, _ := CalculateNetMinutes(s.StartDatetime, s.EndDatetime, s.BreakMinutes)
		agg.netMinutes += net
		if s.HasClip {
			agg.clipCount++
		}
	}

	// 4. Compute pay for each staff
	var rows []domainLive.PayrollRow
	var totalHours float64
	var totalPay float64
	var totalClips int

	for staffID, agg := range staffMap {
		rounded := ApplyRounding(agg.netMinutes, policy)
		decHours := math.Round((float64(rounded)/60.0)*100) / 100

		// Staff specific rate
		hourlyRate := defaultHourlyRate
		if payrollSettings.StaffRates != nil {
			staffIDStr := strconv.FormatUint(uint64(staffID), 10)
			if customRate, ok := payrollSettings.StaffRates[staffIDStr]; ok && customRate > 0 {
				hourlyRate = customRate
			}
		}

		basePay := decHours * float64(hourlyRate)
		clipBonusPay := float64(agg.clipCount * clipBonusRate)
		staffTotalPay := basePay + clipBonusPay

		row := domainLive.PayrollRow{
			StaffID:               staffID,
			StaffName:             agg.staffName,
			ApprovedSessionsCount: agg.sessionsCount,
			TotalNetMinutes:       agg.netMinutes,
			RoundedMinutes:        rounded,
			DecimalHours:          decHours,
			HourlyRate:            hourlyRate,
			BasePay:               basePay,
			ClipBonusCount:        agg.clipCount,
			ClipBonusRate:         clipBonusRate,
			ClipBonusPay:          clipBonusPay,
			TotalPay:              staffTotalPay,
		}

		rows = append(rows, row)
		totalHours += decHours
		totalPay += staffTotalPay
		totalClips += agg.clipCount
	}

	return &domainLive.PayrollSummary{
		Month:          month,
		RoundingPolicy: string(policy),
		TotalHours:     math.Round(totalHours*100) / 100,
		TotalPay:       math.Round(totalPay*100) / 100,
		TotalClips:     totalClips,
		StaffPayroll:   rows,
	}, nil
}

func (u *liveUsecase) CreateContentItem(ctx context.Context, input CreateContentInput) (*domainLive.ContentItem, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", "Title is required", 400)
	}
	kind := input.Kind
	if kind == "" {
		kind = domainLive.ContentKindScheduled
	}

	item := &domainLive.ContentItem{
		Kind:          kind,
		Title:         input.Title,
		Platform:      input.Platform,
		ScheduledFor:  input.ScheduledFor,
		PublishedAt:   input.PublishedAt,
		HostID:        input.HostID,
		HostName:      input.HostName,
		Reach:         input.Reach,
		EngagementPct: input.EngagementPct,
		Notes:         input.Notes,
		CreatedBy:     input.CreatedBy,
	}

	if err := u.liveRepo.CreateContentItem(ctx, item); err != nil {
		return nil, err
	}
	return u.liveRepo.GetContentItemByID(ctx, item.ID)
}

func (u *liveUsecase) UpdateContentItem(ctx context.Context, id uint, input UpdateContentInput) (*domainLive.ContentItem, error) {
	item, err := u.liveRepo.GetContentItemByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Title != "" {
		item.Title = input.Title
	}
	if input.Platform != "" {
		item.Platform = input.Platform
	}
	item.ScheduledFor = input.ScheduledFor
	item.PublishedAt = input.PublishedAt
	item.HostID = input.HostID
	if input.HostName != "" {
		item.HostName = input.HostName
	}
	item.Reach = input.Reach
	item.EngagementPct = input.EngagementPct
	item.Notes = input.Notes

	if err := u.liveRepo.UpdateContentItem(ctx, item); err != nil {
		return nil, err
	}
	return u.liveRepo.GetContentItemByID(ctx, item.ID)
}

func (u *liveUsecase) DeleteContentItem(ctx context.Context, id uint) error {
	return u.liveRepo.DeleteContentItem(ctx, id)
}

func (u *liveUsecase) GetContentItemByID(ctx context.Context, id uint) (*domainLive.ContentItem, error) {
	return u.liveRepo.GetContentItemByID(ctx, id)
}

func (u *liveUsecase) ListContentItems(ctx context.Context, filter domainLive.ContentFilter) ([]domainLive.ContentItem, int64, error) {
	return u.liveRepo.ListContentItems(ctx, filter)
}
