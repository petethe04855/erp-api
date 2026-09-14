package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/internal/delivery/http/dto"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecaseStock "chawy-erp-api/internal/usecase/stock"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStockUsecase struct {
	bySKUItems []domainStock.StockBySKU
	bySKUTotal int64
	bySKUErr   error
}

func (m *mockStockUsecase) GetStock(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error) {
	return nil, nil
}
func (m *mockStockUsecase) ListStock(ctx context.Context, query domainStock.Query) ([]domainStock.Stock, int64, error) {
	return nil, 0, nil
}
func (m *mockStockUsecase) ListStockBySKU(ctx context.Context, query domainStock.StockBySKUQuery) ([]domainStock.StockBySKU, int64, error) {
	return m.bySKUItems, m.bySKUTotal, m.bySKUErr
}
func (m *mockStockUsecase) AdjustStock(ctx context.Context, input usecaseStock.AdjustInput) (*domainStock.Stock, error) {
	return nil, nil
}
func (m *mockStockUsecase) AdjustBySKU(ctx context.Context, skuRepo usecaseStock.SKUResolver, input usecaseStock.AdjustBySKUInput) error {
	return nil
}
func (m *mockStockUsecase) GetMovements(ctx context.Context, skuID uint, page, limit int) ([]domainStock.StockMovement, int64, error) {
	return nil, 0, nil
}

func TestStockHandler_ListStock_ViewBySKU(t *testing.T) {
	mockUC := &mockStockUsecase{
		bySKUItems: []domainStock.StockBySKU{
			{
				SKUID:          1,
				SKUCode:        "SKU-A",
				Quantity:       50,
				ReservedQty:    10,
				AvailableQty:   40,
				WarehouseCount: 2,
			},
			{
				SKUID:          2,
				SKUCode:        "SKU-B",
				Quantity:       100,
				ReservedQty:    0,
				AvailableQty:   100,
				WarehouseCount: 1,
			},
		},
		bySKUTotal: 2,
	}

	h := NewStockHandler(mockUC)
	app := fiber.New()
	app.Get("/api/v1/inventory/stocks", h.ListStock)

	req := httptest.NewRequest("GET", "/api/v1/inventory/stocks?view=by-sku", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var body struct {
		Success bool                     `json:"success"`
		Data    []dto.StockBySKUResponse `json:"data"`
		Meta    struct {
			Page  int   `json:"page"`
			Limit int   `json:"limit"`
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.True(t, body.Success)
	assert.Equal(t, int64(2), body.Meta.Total)
	require.Len(t, body.Data, 2)
	assert.Equal(t, "SKU-A", body.Data[0].SKUCode)
	assert.Equal(t, 40, body.Data[0].AvailableQty)
	assert.Equal(t, 2, body.Data[0].WarehouseCount)
	assert.Equal(t, "SKU-B", body.Data[1].SKUCode)
	assert.Equal(t, 100, body.Data[1].AvailableQty)
}
