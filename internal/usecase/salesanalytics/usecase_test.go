package salesanalytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"chawy-erp-api/internal/domain/salesanalytics"
	"chawy-erp-api/pkg/province"

	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	orders []salesanalytics.Order
	got    salesanalytics.Query
	err    error
}

func (f *fakeRepo) SalesOrders(_ context.Context, q salesanalytics.Query) ([]salesanalytics.Order, error) {
	f.got = q
	if f.err != nil {
		return nil, f.err
	}
	return f.orders, nil
}

func TestSalesAnalyticsValidatesTheFilter(t *testing.T) {
	uc := NewUsecase(&fakeRepo{})

	cases := []struct {
		name string
		q    salesanalytics.Query
		want error
	}{
		{"bad from", salesanalytics.Query{DateFrom: "01/02/2026", DateTo: "2026-11-17"}, ErrInvalidDateFormat},
		{"bad to", salesanalytics.Query{DateFrom: "2026-11-01", DateTo: "tomorrow"}, ErrInvalidDateFormat},
		{"reversed", salesanalytics.Query{DateFrom: "2026-11-17", DateTo: "2026-11-01"}, ErrDateFromAfterDateTo},
		{"too long", salesanalytics.Query{DateFrom: "2026-01-01", DateTo: "2027-01-02"}, ErrDateRangeExceeded},
		{"bad channel", salesanalytics.Query{DateFrom: "2026-11-01", DateTo: "2026-11-17", Channel: "lazada"}, ErrInvalidChannel},
		{"bad region", salesanalytics.Query{DateFrom: "2026-11-01", DateTo: "2026-11-17", Region: "Atlantis"}, ErrInvalidRegion},
	}
	for _, c := range cases {
		_, err := uc.SalesAnalytics(context.Background(), c.q)
		require.ErrorIs(t, err, c.want, c.name)
	}
}

func TestSalesAnalyticsDefaultsAndChannel(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUsecase(repo)

	payload, err := uc.SalesAnalytics(context.Background(), salesanalytics.Query{Channel: "TikTok Live"})
	require.NoError(t, err)
	require.NotNil(t, payload)
	require.Empty(t, payload.Orders, "an empty payload still encodes [] rather than null")
	require.Equal(t, salesanalytics.ChannelTikTok, repo.got.Channel, "content channels map to the tiktok platform")

	from, err := time.ParseInLocation("2006-01-02", repo.got.DateFrom, bangkok)
	require.NoError(t, err)
	to, err := time.ParseInLocation("2006-01-02", repo.got.DateTo, bangkok)
	require.NoError(t, err)
	require.EqualValues(t, DefaultDays, int(to.Sub(from).Hours()/24), "default window is DefaultDays inclusive")

	_, err = uc.SalesAnalytics(context.Background(), salesanalytics.Query{Channel: "all"})
	require.NoError(t, err)
	require.Equal(t, salesanalytics.ChannelAll, repo.got.Channel)
}

func TestSalesAnalyticsFiltersByRegion(t *testing.T) {
	repo := &fakeRepo{orders: []salesanalytics.Order{
		{OrderNo: "A", Province: "Bangkok"},
		{OrderNo: "B", Province: "Chiang Mai"},
		{OrderNo: "C", Province: "Atlantis"},
	}}
	uc := NewUsecase(repo)

	payload, err := uc.SalesAnalytics(context.Background(), salesanalytics.Query{
		DateFrom: "2026-11-01", DateTo: "2026-11-17", Region: province.RegionBangkokMetro,
	})
	require.NoError(t, err)
	require.Len(t, payload.Orders, 1)
	require.Equal(t, "A", payload.Orders[0].OrderNo, "province aliases normalize before region grouping")

	payload, err = uc.SalesAnalytics(context.Background(), salesanalytics.Query{
		DateFrom: "2026-11-01", DateTo: "2026-11-17",
	})
	require.NoError(t, err)
	require.Len(t, payload.Orders, 3, "no region means no filtering")
}

func TestSalesAnalyticsPropagatesRepositoryErrors(t *testing.T) {
	boom := errors.New("db down")
	uc := NewUsecase(&fakeRepo{err: boom})

	_, err := uc.SalesAnalytics(context.Background(), salesanalytics.Query{DateFrom: "2026-11-01", DateTo: "2026-11-17"})
	require.ErrorIs(t, err, boom)
}
