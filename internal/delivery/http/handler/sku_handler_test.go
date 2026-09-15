package handler_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"chawy-erp-api/internal/delivery/http/dto"
	"chawy-erp-api/internal/delivery/http/handler"
	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseSKU "chawy-erp-api/internal/usecase/sku"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSKUUsecase struct {
	itemsWithStats []usecaseSKU.SKUWithStats
	totalStats     int64
	receipts       []domainSKU.SKUReceiptItem
	totalReceipts  int64
}

func (m *mockSKUUsecase) Create(ctx context.Context, input usecaseSKU.CreateInput) (*domainSKU.SKU, error) {
	return nil, nil
}
func (m *mockSKUUsecase) GetByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	return nil, nil
}
func (m *mockSKUUsecase) GetBySKU(ctx context.Context, skuCode string) (*domainSKU.SKU, error) {
	return nil, nil
}
func (m *mockSKUUsecase) List(ctx context.Context, query domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	return nil, 0, nil
}
func (m *mockSKUUsecase) ListWithStats(ctx context.Context, query domainSKU.Query) ([]usecaseSKU.SKUWithStats, int64, error) {
	return m.itemsWithStats, m.totalStats, nil
}
func (m *mockSKUUsecase) GetReceiptHistory(ctx context.Context, query domainSKU.SKUReceiptQuery) ([]domainSKU.SKUReceiptItem, int64, error) {
	return m.receipts, m.totalReceipts, nil
}
func (m *mockSKUUsecase) Update(ctx context.Context, id uint, input usecaseSKU.UpdateInput) (*domainSKU.SKU, error) {
	return nil, nil
}
func (m *mockSKUUsecase) Delete(ctx context.Context, id uint) error {
	return nil
}

func TestSKUHandler_List_IncludesReceiptStats(t *testing.T) {
	now := time.Now()
	mockUC := &mockSKUUsecase{
		itemsWithStats: []usecaseSKU.SKUWithStats{
			{
				SKU: domainSKU.SKU{
					ID:        1,
					SKU:       "CHICKEN-001",
					Name:      "Chicken Breast",
					CreatedAt: now.Add(-24 * time.Hour),
				},
				LastReceivedAt: &now,
				ReceiptCount:   3,
			},
		},
		totalStats: 1,
	}

	h := handler.NewSKUHandler(mockUC)
	app := fiber.New()
	app.Get("/api/v1/skus", h.List)

	req := httptest.NewRequest("GET", "/api/v1/skus", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var body struct {
		Success bool              `json:"success"`
		Data    []dto.SKUResponse `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	require.Len(t, body.Data, 1)
	assert.Equal(t, "CHICKEN-001", body.Data[0].SKU)
	assert.Equal(t, 3, body.Data[0].ReceiptCount)
	assert.NotNil(t, body.Data[0].LastReceivedAt)
}

func TestSKUHandler_GetReceiptHistory(t *testing.T) {
	recTime := time.Date(2026, 9, 15, 10, 30, 0, 0, time.UTC)
	mockUC := &mockSKUUsecase{
		receipts: []domainSKU.SKUReceiptItem{
			{
				ID:               101,
				ReceivedAt:       recTime,
				SourceType:       "GOODS_RECEIVE",
				Quantity:         20,
				WarehouseID:      1,
				WarehouseName:    "คลังหลัก",
				LotNumber:        "LOT-20260915-01",
				SupplierLot:      "SUP-01",
				ExpiryDate:       "2027-03-15",
				ReferenceType:    "GOODS_RECEIVE",
				ReferenceID:      "GR-2026-09-15-001",
				PurchaseOrderRef: "PO-2026-09-10-001",
				Note:             "Regular receipt",
			},
		},
		totalReceipts: 1,
	}

	h := handler.NewSKUHandler(mockUC)
	app := fiber.New()
	app.Get("/api/v1/skus/:id/receipts", h.GetReceiptHistory)

	req := httptest.NewRequest("GET", "/api/v1/skus/1/receipts?page=1&limit=10", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var body struct {
		Success bool                     `json:"success"`
		Data    []dto.SKUReceiptResponse `json:"data"`
		Meta    struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	require.Len(t, body.Data, 1)
	assert.Equal(t, uint(101), body.Data[0].ID)
	assert.Equal(t, "GOODS_RECEIVE", body.Data[0].SourceType)
	assert.Equal(t, 20, body.Data[0].Quantity)
	assert.Equal(t, "GR-2026-09-15-001", body.Data[0].ReferenceID)
	assert.Equal(t, "LOT-20260915-01", body.Data[0].LotNumber)
}
