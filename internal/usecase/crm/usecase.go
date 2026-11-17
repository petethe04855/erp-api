package crm

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"chawy-erp-api/internal/domain/crm"
	"chawy-erp-api/pkg/province"
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

	report, err := u.crmRepo.GetTiktokProvinceReport(ctx, validatedQuery)
	if err != nil {
		return nil, err
	}
	if report == nil {
		report = &crm.TiktokProvinceReport{}
	}
	enrichProvinceReport(report)

	daily, err := u.crmRepo.GetDailyChannelSales(ctx, validatedQuery)
	if err != nil {
		return nil, err
	}
	report.Trend = buildSalesTrend(daily, dateFrom, dateTo)
	return report, nil
}

// enrichProvinceReport derives region, AOV, sales share, coverage and the
// per-region breakdown from the aggregated province rows.
func enrichProvinceReport(report *crm.TiktokProvinceReport) {
	if report == nil {
		return
	}

	totalProvinces := province.TotalProvinces()
	report.Summary.TotalProvinces = totalProvinces
	if report.Summary.TotalOrders > 0 {
		report.Summary.AvgOrderValue = round2(report.Summary.GrossSales / float64(report.Summary.TotalOrders))
	}
	if totalProvinces > 0 {
		report.Summary.CoveragePercent = round2(float64(report.Summary.ProvinceCount) / float64(totalProvinces) * 100)
	}

	regionMap := make(map[string]*crm.ProvinceRegionRow)
	for i := range report.Provinces {
		row := &report.Provinces[i]
		row.Region = province.RegionOf(row.Province)
		if row.OrderCount > 0 {
			row.AvgOrderValue = round2(row.GrossSales / float64(row.OrderCount))
		}
		if report.Summary.GrossSales > 0 {
			row.SalesSharePercent = round2(row.GrossSales / report.Summary.GrossSales * 100)
		}

		reg, ok := regionMap[row.Region]
		if !ok {
			reg = &crm.ProvinceRegionRow{Region: row.Region}
			regionMap[row.Region] = reg
		}
		if row.Province != province.UnknownProvince {
			reg.ProvinceCount++
		}
		reg.OrderCount += row.OrderCount
		reg.GrossSales += row.GrossSales
	}

	regions := make([]crm.ProvinceRegionRow, 0, len(regionMap))
	for _, reg := range regionMap {
		reg.GrossSales = round2(reg.GrossSales)
		if report.Summary.TotalOrders > 0 {
			reg.SharePercent = round2(float64(reg.OrderCount) / float64(report.Summary.TotalOrders) * 100)
		}
		regions = append(regions, *reg)
	}
	sort.SliceStable(regions, func(i, j int) bool {
		if regions[i].OrderCount != regions[j].OrderCount {
			return regions[i].OrderCount > regions[j].OrderCount
		}
		return regions[i].Region < regions[j].Region
	})
	report.Regions = regions
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
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
