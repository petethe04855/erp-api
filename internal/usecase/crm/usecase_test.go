package crm_test

import (
	"context"
	"testing"

	"chawy-erp-api/internal/domain/crm"
	usecaseCRM "chawy-erp-api/internal/usecase/crm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCRMRepo struct {
	lastQuery crm.ProvinceQuery
	report    *crm.TiktokProvinceReport
	daily     []crm.DailyChannelSales
	err       error
}

func (m *mockCRMRepo) GetTiktokProvinceReport(ctx context.Context, query crm.ProvinceQuery) (*crm.TiktokProvinceReport, error) {
	m.lastQuery = query
	return m.report, m.err
}

func (m *mockCRMRepo) GetDailyChannelSales(ctx context.Context, query crm.ProvinceQuery) ([]crm.DailyChannelSales, error) {
	return m.daily, nil
}

func (m *mockCRMRepo) SearchProvinceReport(ctx context.Context, query crm.ProvinceSearchRequest) (*crm.TiktokProvinceReport, error) {
	return m.report, m.err
}

func TestGetTiktokProvinceReport_Validation(t *testing.T) {
	mockRepo := &mockCRMRepo{
		report: &crm.TiktokProvinceReport{},
	}
	uc := usecaseCRM.NewUsecase(mockRepo)
	ctx := context.Background()

	t.Run("DateFrom after DateTo", func(t *testing.T) {
		_, err := uc.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{
			DateFrom: "2026-10-10",
			DateTo:   "2026-10-01",
		})
		assert.ErrorIs(t, err, usecaseCRM.ErrDateFromAfterDateTo)
	})

	t.Run("Date range exceeds 366 days", func(t *testing.T) {
		_, err := uc.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{
			DateFrom: "2025-01-01",
			DateTo:   "2026-02-01",
		})
		assert.ErrorIs(t, err, usecaseCRM.ErrDateRangeExceeded)
	})

	t.Run("Invalid date format", func(t *testing.T) {
		_, err := uc.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{
			DateFrom: "invalid-date",
			DateTo:   "2026-10-01",
		})
		assert.ErrorIs(t, err, usecaseCRM.ErrInvalidDateFormat)
	})

	t.Run("Default 30 days applied when dates empty", func(t *testing.T) {
		_, err := uc.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{})
		require.NoError(t, err)
		assert.NotEmpty(t, mockRepo.lastQuery.DateFrom)
		assert.NotEmpty(t, mockRepo.lastQuery.DateTo)
		assert.True(t, mockRepo.lastQuery.DateFrom <= mockRepo.lastQuery.DateTo)
	})

	t.Run("Valid custom dates", func(t *testing.T) {
		_, err := uc.GetTiktokProvinceReport(ctx, crm.ProvinceQuery{
			DateFrom: "2026-09-01",
			DateTo:   "2026-09-30",
			Status:   "fulfilled",
			Province: "กรุงเทพมหานคร",
		})
		require.NoError(t, err)
		assert.Equal(t, "2026-09-01", mockRepo.lastQuery.DateFrom)
		assert.Equal(t, "2026-09-30", mockRepo.lastQuery.DateTo)
		assert.Equal(t, "fulfilled", mockRepo.lastQuery.Status)
		assert.Equal(t, "กรุงเทพมหานคร", mockRepo.lastQuery.Province)
	})
}

func TestGetTiktokProvinceReport_EnrichesRegionsAndAOV(t *testing.T) {
	mockRepo := &mockCRMRepo{
		report: &crm.TiktokProvinceReport{
			Summary: crm.TiktokProvinceSummary{TotalOrders: 10, GrossSales: 2000, ProvinceCount: 2},
			Provinces: []crm.TiktokProvinceRow{
				{Province: "กรุงเทพมหานคร", OrderCount: 6, GrossSales: 1500},
				{Province: "ชลบุรี", OrderCount: 3, GrossSales: 400},
				{Province: "ไม่ทราบจังหวัด", OrderCount: 1, GrossSales: 100},
			},
		},
	}
	uc := usecaseCRM.NewUsecase(mockRepo)

	report, err := uc.GetTiktokProvinceReport(context.Background(), crm.ProvinceQuery{
		DateFrom: "2026-09-01",
		DateTo:   "2026-09-30",
	})
	assert.NoError(t, err)
	assert.Equal(t, 200.0, report.Summary.AvgOrderValue)
	assert.Equal(t, 77, report.Summary.TotalProvinces)
	assert.Equal(t, 2.6, report.Summary.CoveragePercent)

	assert.Equal(t, "กรุงเทพฯ และปริมณฑล", report.Provinces[0].Region)
	assert.Equal(t, 250.0, report.Provinces[0].AvgOrderValue)
	assert.Equal(t, 75.0, report.Provinces[0].SalesSharePercent)
	assert.Equal(t, "ภาคตะวันออก", report.Provinces[1].Region)
	assert.Equal(t, "ไม่ระบุภูมิภาค", report.Provinces[2].Region)

	assert.Len(t, report.Regions, 3)
	assert.Equal(t, "กรุงเทพฯ และปริมณฑล", report.Regions[0].Region)
	assert.Equal(t, 60.0, report.Regions[0].SharePercent)
	assert.Equal(t, 0, report.Regions[2].ProvinceCount)
}
