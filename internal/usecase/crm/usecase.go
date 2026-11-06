package crm

import (
	"context"
	"errors"
	"strings"
	"time"

	"chawy-erp-api/internal/domain/crm"
)

var (
	ErrInvalidDateFormat   = errors.New("invalid date format, must be YYYY-MM-DD")
	ErrDateFromAfterDateTo = errors.New("dateFrom cannot be after dateTo")
	ErrDateRangeExceeded   = errors.New("date range cannot exceed 366 days")
)

type Usecase interface {
	GetTiktokProvinceReport(ctx context.Context, query crm.ProvinceQuery) (*crm.TiktokProvinceReport, error)
	SearchProvinceReport(ctx context.Context, req crm.ProvinceSearchRequest) (*crm.TiktokProvinceReport, error)
}

type crmUsecase struct {
	crmRepo crm.Repository
}

func NewUsecase(crmRepo crm.Repository) Usecase {
	return &crmUsecase{crmRepo: crmRepo}
}

func (u *crmUsecase) GetTiktokProvinceReport(ctx context.Context, query crm.ProvinceQuery) (*crm.TiktokProvinceReport, error) {
	loc := time.FixedZone("Asia/Bangkok", 7*60*60)
	now := time.Now().In(loc)

	dateFromStr := strings.TrimSpace(query.DateFrom)
	dateToStr := strings.TrimSpace(query.DateTo)

	var dateFrom, dateTo time.Time
	var err error

	if dateToStr != "" {
		dateTo, err = time.ParseInLocation("2006-01-02", dateToStr, loc)
		if err != nil {
			return nil, ErrInvalidDateFormat
		}
	} else {
		dateTo = now
		dateToStr = dateTo.Format("2006-01-02")
	}

	if dateFromStr != "" {
		dateFrom, err = time.ParseInLocation("2006-01-02", dateFromStr, loc)
		if err != nil {
			return nil, ErrInvalidDateFormat
		}
	} else {
		dateFrom = dateTo.AddDate(0, 0, -30)
		dateFromStr = dateFrom.Format("2006-01-02")
	}

	if dateFrom.After(dateTo) {
		return nil, ErrDateFromAfterDateTo
	}

	if dateTo.Sub(dateFrom) > 366*24*time.Hour {
		return nil, ErrDateRangeExceeded
	}

	validatedQuery := crm.ProvinceQuery{
		DateFrom: dateFromStr,
		DateTo:   dateToStr,
		Status:   query.Status,
		Province: query.Province,
		Channel:  query.Channel,
	}

	return u.crmRepo.GetTiktokProvinceReport(ctx, validatedQuery)
}

func (u *crmUsecase) SearchProvinceReport(ctx context.Context, req crm.ProvinceSearchRequest) (*crm.TiktokProvinceReport, error) {
	province := ""
	if len(req.Province) > 0 {
		province = req.Province[0]
	}

	channel := strings.ToLower(strings.TrimSpace(req.Channel))
	if channel == "" {
		channel = "all"
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "fulfilled"
	}

	query := crm.ProvinceQuery{
		Channel:  channel,
		DateFrom: req.DateFrom,
		DateTo:   req.DateTo,
		Status:   status,
		Province: province,
	}

	return u.GetTiktokProvinceReport(ctx, query)
}
