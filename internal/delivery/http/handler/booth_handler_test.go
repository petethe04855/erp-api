package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	domainBooth "chawy-erp-api/internal/domain/booth"
	usecaseBooth "chawy-erp-api/internal/usecase/booth"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type stubBooth struct {
	usecaseBooth.Usecase
	got    usecaseBooth.SaleInput
	result *usecaseBooth.SaleResult
	err    error
}

func (s *stubBooth) CreateSale(_ context.Context, in usecaseBooth.SaleInput) (*usecaseBooth.SaleResult, error) {
	s.got = in
	return s.result, s.err
}

func boothApp(uc usecaseBooth.Usecase) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", uint(7))
		c.Locals("email", "sales@chawy")
		return c.Next()
	})
	app.Post("/booth/sales", NewBoothHandler(uc).CreateSale)
	return app
}

func postSale(t *testing.T, app *fiber.App) (int, map[string]any) {
	req := httptest.NewRequest("POST", "/booth/sales", strings.NewReader(
		`{"clientRef":"dev-1","paymentMethod":"cash","cashReceived":500,"lines":[{"skuId":11,"qty":2}]}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(res.Body)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	return res.StatusCode, body
}

func TestBoothCreateSaleUsesTheLoggedInSeller(t *testing.T) {
	stub := &stubBooth{result: &usecaseBooth.SaleResult{Sale: &domainBooth.Sale{ID: 1, SaleNo: "BTH-000001", Total: 258}}}
	code, body := postSale(t, boothApp(stub))
	require.Equal(t, 201, code)
	require.Equal(t, uint(7), stub.got.SellerID)
	require.Equal(t, "sales@chawy", stub.got.SellerName)
	require.Equal(t, 500.0, *stub.got.CashReceived)
	require.Equal(t, []usecaseBooth.CartLine{{SKUID: 11, Qty: 2}}, stub.got.Lines)
	sale := body["data"].(map[string]any)["sale"].(map[string]any)
	require.Equal(t, "BTH-000001", sale["saleNo"])

	stub.result.Replayed = true
	code, _ = postSale(t, boothApp(stub))
	require.Equal(t, 200, code)
}

func TestBoothCreateSaleShortageIs409WithEveryShortSKU(t *testing.T) {
	stub := &stubBooth{err: &usecaseBooth.InsufficientStockError{Shortages: []usecaseBooth.Shortage{
		{SKUID: 11, SKUCode: "TRT-CHK", Name: "ไก่", Requested: 2, Available: 1},
	}}}
	code, body := postSale(t, boothApp(stub))
	require.Equal(t, 409, code)
	require.Equal(t, false, body["success"])
	require.Equal(t, "INSUFFICIENT_STOCK", body["error"].(map[string]any)["code"])
	shortages := body["data"].(map[string]any)["shortages"].([]any)
	require.Len(t, shortages, 1)
}
