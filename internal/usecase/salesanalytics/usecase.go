// Package salesanalytics validates CRM dashboard requests and returns the
// order facts of a range. All KPI maths lives in crm-core; this layer only
// enforces the date window, the channel and the region filter.
package salesanalytics

import (
	"context"
	"errors"
	"strings"
	"time"

	"chawy-erp-api/internal/domain/salesanalytics"
	"chawy-erp-api/pkg/province"
)

// Validation errors returned to the delivery layer as 400s.
var (
	ErrInvalidDateFormat   = errors.New("invalid date format, must be YYYY-MM-DD")
	ErrDateFromAfterDateTo = errors.New("dateFrom cannot be after dateTo")
	ErrDateRangeExceeded   = errors.New("date range cannot exceed 366 days")
	ErrInvalidChannel      = errors.New("channel must be all, shopee or tiktok")
	ErrInvalidRegion       = errors.New("unknown region")
)

// Bangkok is the business time zone of the dashboard.
var bangkok = time.FixedZone("Asia/Bangkok", 7*60*60)

// DefaultDays is the range used when the caller sends no dates; it matches the
// backoffice default so both layers agree.
const DefaultDays = 90

// MaxDays caps one request to a year plus a day.
const MaxDays = 366

// Usecase serves GET /crm/sales-analytics.
type Usecase interface {
	SalesAnalytics(ctx context.Context, q salesanalytics.Query) (*salesanalytics.Payload, error)
}

type usecase struct {
	repo salesanalytics.Repository
}

// NewUsecase builds the usecase.
func NewUsecase(repo salesanalytics.Repository) Usecase {
	return &usecase{repo: repo}
}

// SalesAnalytics validates the filter, loads the facts and applies the region
// grouping. The payload is never nil so the response always encodes a list.
func (u *usecase) SalesAnalytics(ctx context.Context, q salesanalytics.Query) (*salesanalytics.Payload, error) {
	from, to, err := parseRange(q.DateFrom, q.DateTo)
	if err != nil {
		return nil, err
	}
	channel, err := parseChannel(q.Channel)
	if err != nil {
		return nil, err
	}
	region := strings.TrimSpace(q.Region)
	if region != "" && !isKnownRegion(region) {
		return nil, ErrInvalidRegion
	}

	orders, err := u.repo.SalesOrders(ctx, salesanalytics.Query{
		DateFrom: from.Format("2006-01-02"),
		DateTo:   to.Format("2006-01-02"),
		Channel:  channel,
	})
	if err != nil {
		return nil, err
	}
	if orders == nil {
		orders = []salesanalytics.Order{}
	}

	if region != "" {
		filtered := make([]salesanalytics.Order, 0, len(orders))
		for _, o := range orders {
			name := province.NormalizeProvince(o.Province, "")
			if province.RegionOf(name) == region {
				filtered = append(filtered, o)
			}
		}
		orders = filtered
	}
	return &salesanalytics.Payload{Orders: orders}, nil
}

// parseRange validates the inclusive date window. Missing bounds fall back to
// the last DefaultDays ending today (Bangkok).
func parseRange(rawFrom, rawTo string) (time.Time, time.Time, error) {
	now := time.Now().In(bangkok)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, bangkok)

	if v := strings.TrimSpace(rawTo); v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, bangkok)
		if err != nil {
			return time.Time{}, time.Time{}, ErrInvalidDateFormat
		}
		to = parsed
	}

	from := to.AddDate(0, 0, -DefaultDays)
	if v := strings.TrimSpace(rawFrom); v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, bangkok)
		if err != nil {
			return time.Time{}, time.Time{}, ErrInvalidDateFormat
		}
		from = parsed
	}

	if from.After(to) {
		return time.Time{}, time.Time{}, ErrDateFromAfterDateTo
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > MaxDays {
		return time.Time{}, time.Time{}, ErrDateRangeExceeded
	}
	return from, to, nil
}

func parseChannel(raw string) (string, error) {
	channel := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(raw)), " ", "_")
	switch channel {
	case "", salesanalytics.ChannelAll:
		return salesanalytics.ChannelAll, nil
	case salesanalytics.ChannelShopee:
		return salesanalytics.ChannelShopee, nil
	case salesanalytics.ChannelTikTok, "tiktok_live", "tiktok_product", "tiktok_video":
		return salesanalytics.ChannelTikTok, nil
	default:
		return "", ErrInvalidChannel
	}
}

func isKnownRegion(name string) bool {
	for _, r := range province.Regions() {
		if r == name {
			return true
		}
	}
	return false
}
