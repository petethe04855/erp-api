package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/internal/domain/salesanalytics"
	usecaseSA "chawy-erp-api/internal/usecase/salesanalytics"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type fakeSalesAnalytics struct {
	payload *salesanalytics.Payload
	err     error
	got     salesanalytics.Query
}

func (f *fakeSalesAnalytics) SalesAnalytics(_ context.Context, q salesanalytics.Query) (*salesanalytics.Payload, error) {
	f.got = q
	if f.err != nil {
		return nil, f.err
	}
	return f.payload, nil
}

func newSalesAnalyticsApp(uc usecaseSA.Usecase) *fiber.App {
	app := fiber.New()
	h := NewSalesAnalyticsHandler(uc)
	app.Get("/api/v1/crm/sales-analytics", h.GetSalesAnalytics)
	return app
}

func TestSalesAnalyticsHandlerReturnsFacts(t *testing.T) {
	fake := &fakeSalesAnalytics{payload: &salesanalytics.Payload{Orders: []salesanalytics.Order{
		{Platform: "shopee", OrderNo: "A", Date: "2026-11-02"},
	}}}
	app := newSalesAnalyticsApp(fake)

	resp, err := app.Test(httptest.NewRequest("GET",
		"/api/v1/crm/sales-analytics?dateFrom=2026-11-01&dateTo=2026-11-30&channel=tiktok_live&region=%E0%B8%A0%E0%B8%B2%E0%B8%84%E0%B9%83%E0%B8%95%E0%B9%89", nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Orders []struct {
				OrderNo string `json:"orderNo"`
			} `json:"orders"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.True(t, body.Success)
	require.Len(t, body.Data.Orders, 1)
	require.Equal(t, "A", body.Data.Orders[0].OrderNo)

	require.Equal(t, "2026-11-01", fake.got.DateFrom)
	require.Equal(t, "tiktok_live", fake.got.Channel)
	require.Equal(t, "ภาคใต้", fake.got.Region)
}

func TestSalesAnalyticsHandlerMapsValidationErrors(t *testing.T) {
	app := newSalesAnalyticsApp(&fakeSalesAnalytics{err: usecaseSA.ErrInvalidDateFormat})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/crm/sales-analytics?dateFrom=01/11/2026", nil))
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)

	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.False(t, body.Success)
	require.Equal(t, "VALIDATION_ERROR", body.Error.Code)
}

func TestSalesAnalyticsHandlerHidesInternalErrors(t *testing.T) {
	app := newSalesAnalyticsApp(&fakeSalesAnalytics{err: errors.New("db down")})

	resp, err := app.Test(httptest.NewRequest("GET",
		"/api/v1/crm/sales-analytics?dateFrom=2026-11-01&dateTo=2026-11-30", nil))
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode)

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotContains(t, body.Error.Message, "db down")
}
