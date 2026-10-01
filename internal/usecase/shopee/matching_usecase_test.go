package shopee_test

import (
	"context"
	"testing"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseShopee "chawy-erp-api/internal/usecase/shopee"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockCostUsecase struct {
	mock.Mock
}

func (m *mockCostUsecase) AddCostHistory(ctx context.Context, skuCode string, costPrice float64, effectiveFrom string, effectiveTo *string, note string) (*domainSKU.SKUCostHistory, error) {
	args := m.Called(ctx, skuCode, costPrice, effectiveFrom, effectiveTo, note)
	return args.Get(0).(*domainSKU.SKUCostHistory), args.Error(1)
}

func (m *mockCostUsecase) GetCostHistoryBySKU(ctx context.Context, skuCode string) ([]domainSKU.SKUCostHistory, error) {
	args := m.Called(ctx, skuCode)
	return args.Get(0).([]domainSKU.SKUCostHistory), args.Error(1)
}

func (m *mockCostUsecase) GetEffectiveCost(ctx context.Context, skuCode string, date time.Time) (float64, error) {
	args := m.Called(ctx, skuCode, date)
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockCostUsecase) DeleteCostHistory(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestMatchingUsecase_MultiItemAllocationAndFormulas(t *testing.T) {
	ctx := context.Background()
	incomeRepo := new(mockShopeeIncomeRepo)
	orderRepo := new(mockShopeeOrderRepo)
	costUsecase := new(mockCostUsecase)

	uc := usecaseShopee.NewMatchingUsecase(incomeRepo, orderRepo, costUsecase)

	transferDate, _ := time.Parse("2006-01-02", "2026-03-05")
	orderDate, _ := time.Parse("2006-01-02", "2026-03-01")

	// 1 Order with 2 items:
	// Item 1: LineSale = 100, Qty = 2, Unit Cost = 30 -> Total Cost = 60
	// Item 2: LineSale = 50,  Qty = 1, Unit Cost = 20 -> Total Cost = 20
	// Total Gross = 150
	// Income Net = 135 (Total Fee = 15)
	// Allocated Net:
	// Item 1: 135 * (100 / 150) = 90.00
	// Item 2: 135 * (50 / 150)  = 45.00
	// Profit:
	// Item 1: 90 - 60 = 30.00
	// Item 2: 45 - 20 = 25.00
	// Total Net Profit = 55.00

	incomes := []domainShopee.ShopeeIncome{
		{
			ID:           1,
			OrderID:      "ORDER-001",
			TransferDate: transferDate,
			NetAmount:    135.00,
		},
	}

	orders := []domainShopee.ShopeeOrder{
		{
			ID:        "ORDER-001",
			OrderDate: orderDate,
			Items: []domainShopee.ShopeeOrderItem{
				{
					ID:          101,
					OrderID:     "ORDER-001",
					SKU:         "SKU-A",
					ProductName: "Item A",
					Qty:         2,
					SalePrice:   100.00, // BR-01: Line total
				},
				{
					ID:          102,
					OrderID:     "ORDER-001",
					SKU:         "SKU-B",
					ProductName: "Item B",
					Qty:         1,
					SalePrice:   50.00,
				},
			},
		},
	}

	incomeRepo.On("FindByMonth", ctx, 2026, 3).Return(incomes, nil).Once()
	orderRepo.On("FindByOrderIDs", ctx, []string{"ORDER-001"}).Return(orders, nil).Once()

	costUsecase.On("GetEffectiveCost", ctx, "SKU-A", orderDate).Return(30.00, nil).Once()
	costUsecase.On("GetEffectiveCost", ctx, "SKU-B", orderDate).Return(20.00, nil).Once()

	rows, summary, err := uc.GetMonthlyMatching(ctx, 2026, 3)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(rows))

	// Verify Summary
	assert.Equal(t, 150.00, summary.GrossSale)
	assert.Equal(t, 15.00, summary.PlatformFees)
	assert.Equal(t, 135.00, summary.NetReceive)
	assert.Equal(t, 80.00, summary.TotalCost)
	assert.Equal(t, 55.00, summary.NetProfit)

	// Verify Item 1 allocations
	assert.Equal(t, 100.00, rows[0].LineSale)
	assert.Equal(t, 60.00, rows[0].TotalCost)
	assert.Equal(t, 90.00, rows[0].AllocatedNet)
	assert.Equal(t, 10.00, rows[0].AllocatedFee)
	assert.Equal(t, 30.00, rows[0].Profit)

	// Verify Item 2 allocations
	assert.Equal(t, 50.00, rows[1].LineSale)
	assert.Equal(t, 20.00, rows[1].TotalCost)
	assert.Equal(t, 45.00, rows[1].AllocatedNet)
	assert.Equal(t, 5.00, rows[1].AllocatedFee)
	assert.Equal(t, 25.00, rows[1].Profit)

	// Test CSV Export
	incomeRepo.On("FindByMonth", ctx, 2026, 3).Return(incomes, nil).Once()
	orderRepo.On("FindByOrderIDs", ctx, []string{"ORDER-001"}).Return(orders, nil).Once()
	costUsecase.On("GetEffectiveCost", ctx, "SKU-A", orderDate).Return(30.00, nil).Once()
	costUsecase.On("GetEffectiveCost", ctx, "SKU-B", orderDate).Return(20.00, nil).Once()

	csvBytes, filename, err := uc.ExportMonthlyMatchingCSV(ctx, 2026, 3)
	assert.NoError(t, err)
	assert.Equal(t, "shopee_matching_2026_03.csv", filename)
	assert.Contains(t, string(csvBytes), "ORDER-001")
	assert.Contains(t, string(csvBytes), "SKU-A")
}
