package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/internal/delivery/http/middleware"
	domainCRMIntegration "chawy-erp-api/internal/domain/crmintegration"
	usecaseCRMIntegration "chawy-erp-api/internal/usecase/crmintegration"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type fakeCRMIntegration struct {
	stockIDs []uint
	issue    usecaseCRMIntegration.GoodsIssueInput
	short    bool
	replayed bool
}

func (f *fakeCRMIntegration) SearchSKUs(_ context.Context, search string, wh uint, page, limit int) ([]usecaseCRMIntegration.SKUItem, int64, error) {
	return []usecaseCRMIntegration.SKUItem{{ID: 1, Code: "CHK-01", Name: search, AvailableQty: 8}}, 1, nil
}

func (f *fakeCRMIntegration) Stock(_ context.Context, ids []uint, wh uint) ([]usecaseCRMIntegration.StockItem, error) {
	f.stockIDs = ids
	return []usecaseCRMIntegration.StockItem{{SKUID: 1, WarehouseID: 1, AvailableQty: 8}}, nil
}

func (f *fakeCRMIntegration) CreateGoodsIssue(_ context.Context, in usecaseCRMIntegration.GoodsIssueInput) (*usecaseCRMIntegration.GoodsIssueResult, error) {
	f.issue = in
	if f.short {
		return nil, &usecaseCRMIntegration.InsufficientStockError{Shortages: []usecaseCRMIntegration.Shortage{{SKUID: 1, SKUCode: "CHK-01", Requested: 9, Available: 8}}}
	}
	return &usecaseCRMIntegration.GoodsIssueResult{GIRef: "CRMGI-000001", Status: "issued", Replayed: f.replayed,
		Items: []domainCRMIntegration.GoodsIssueItem{{SKUID: 1, SKUCode: "CHK-01", Qty: 1}}}, nil
}

func newCRMIntegrationApp(token string, uc usecaseCRMIntegration.Usecase) *fiber.App {
	app := fiber.New()
	h := NewCRMIntegrationHandler(uc)
	g := app.Group("/api/v1/integrations/crm", middleware.ServiceToken(token))
	g.Get("/skus", h.SearchSKUs)
	g.Get("/stock", h.Stock)
	g.Post("/goods-issues", h.CreateGoodsIssue)
	return app
}

type integrationRes struct {
	status int
	body   struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
}

func callIntegration(t *testing.T, app *fiber.App, method, path, bearer string, body any) integrationRes {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := integrationRes{status: resp.StatusCode}
	require.NoError(t, json.Unmarshal(raw, &out.body), string(raw))
	return out
}

const testServiceToken = "crm-service-token-for-tests-0123456789"

func TestCRMIntegrationServiceToken(t *testing.T) {
	app := newCRMIntegrationApp(testServiceToken, &fakeCRMIntegration{})
	res := callIntegration(t, app, http.MethodGet, "/api/v1/integrations/crm/skus", "", nil)
	require.Equal(t, http.StatusUnauthorized, res.status)
	require.Equal(t, "MISSING_TOKEN", res.body.Error.Code)
	res = callIntegration(t, app, http.MethodGet, "/api/v1/integrations/crm/skus", "wrong", nil)
	require.Equal(t, http.StatusUnauthorized, res.status)
	require.Equal(t, "INVALID_TOKEN", res.body.Error.Code)
	res = callIntegration(t, app, http.MethodGet, "/api/v1/integrations/crm/skus?search=ไก่", testServiceToken, nil)
	require.Equal(t, http.StatusOK, res.status)

	closed := newCRMIntegrationApp("", &fakeCRMIntegration{})
	res = callIntegration(t, closed, http.MethodGet, "/api/v1/integrations/crm/skus", testServiceToken, nil)
	require.Equal(t, http.StatusServiceUnavailable, res.status, "an unconfigured token fails closed")
	require.Equal(t, "INTEGRATION_DISABLED", res.body.Error.Code)
}

func TestCRMIntegrationStockParsesIDs(t *testing.T) {
	uc := &fakeCRMIntegration{}
	app := newCRMIntegrationApp(testServiceToken, uc)
	res := callIntegration(t, app, http.MethodGet, "/api/v1/integrations/crm/stock?skuIds=1,%202,,3", testServiceToken, nil)
	require.Equal(t, http.StatusOK, res.status)
	require.Equal(t, []uint{1, 2, 3}, uc.stockIDs)

	res = callIntegration(t, app, http.MethodGet, "/api/v1/integrations/crm/stock?skuIds=1,x", testServiceToken, nil)
	require.Equal(t, http.StatusBadRequest, res.status)
	res = callIntegration(t, app, http.MethodGet, "/api/v1/integrations/crm/stock?skuIds=1&warehouseId=-1", testServiceToken, nil)
	require.Equal(t, http.StatusBadRequest, res.status)
}

func TestCRMIntegrationGoodsIssueResponses(t *testing.T) {
	uc := &fakeCRMIntegration{}
	app := newCRMIntegrationApp(testServiceToken, uc)
	body := map[string]any{"idempotencyKey": "RD-1", "warehouseId": 1, "reference": "RD-1",
		"items": []map[string]any{{"skuId": 1, "qty": 1}}}

	res := callIntegration(t, app, http.MethodPost, "/api/v1/integrations/crm/goods-issues", testServiceToken, body)
	require.Equal(t, http.StatusCreated, res.status)
	require.Equal(t, "RD-1", uc.issue.IdempotencyKey)
	require.Equal(t, uint(1), uc.issue.Items[0].SKUID)
	var gi struct {
		GIRef string `json:"giRef"`
	}
	require.NoError(t, json.Unmarshal(res.body.Data, &gi))
	require.Equal(t, "CRMGI-000001", gi.GIRef)

	uc.replayed = true
	res = callIntegration(t, app, http.MethodPost, "/api/v1/integrations/crm/goods-issues", testServiceToken, body)
	require.Equal(t, http.StatusOK, res.status, "a replay answers 200")

	uc.short = true
	res = callIntegration(t, app, http.MethodPost, "/api/v1/integrations/crm/goods-issues", testServiceToken, body)
	require.Equal(t, http.StatusConflict, res.status)
	require.Equal(t, "INSUFFICIENT_STOCK", res.body.Error.Code)
	var short struct {
		Shortages []usecaseCRMIntegration.Shortage `json:"shortages"`
	}
	require.NoError(t, json.Unmarshal(res.body.Data, &short))
	require.Equal(t, 8, short.Shortages[0].Available)
}
