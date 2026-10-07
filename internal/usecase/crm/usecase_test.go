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
	err       error
}

func (m *mockCRMRepo) GetTiktokProvinceReport(ctx context.Context, query crm.ProvinceQuery) (*crm.TiktokProvinceReport, error) {
	m.lastQuery = query
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
