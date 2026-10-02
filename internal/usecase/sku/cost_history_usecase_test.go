package sku_test

import (
	"context"
	"testing"
	"time"

	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseSKU "chawy-erp-api/internal/usecase/sku"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockCostHistoryRepo struct {
	mock.Mock
}

func (m *mockCostHistoryRepo) Create(ctx context.Context, item *domainSKU.SKUCostHistory) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *mockCostHistoryRepo) FindBySKU(ctx context.Context, sku string) ([]domainSKU.SKUCostHistory, error) {
	args := m.Called(ctx, sku)
	return args.Get(0).([]domainSKU.SKUCostHistory), args.Error(1)
}

func (m *mockCostHistoryRepo) FindEffectiveCost(ctx context.Context, sku string, date time.Time) (float64, bool, error) {
	args := m.Called(ctx, sku, date)
	return args.Get(0).(float64), args.Bool(1), args.Error(2)
}

func (m *mockCostHistoryRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockSKURepo struct {
	mock.Mock
}

func (m *mockSKURepo) Create(ctx context.Context, item *domainSKU.SKU) error {
	return m.Called(ctx, item).Error(0)
}
func (m *mockSKURepo) FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainSKU.SKU), args.Error(1)
}
func (m *mockSKURepo) FindBySKU(ctx context.Context, sku string) (*domainSKU.SKU, error) {
	args := m.Called(ctx, sku)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainSKU.SKU), args.Error(1)
}
func (m *mockSKURepo) FindAll(ctx context.Context, query domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]domainSKU.SKU), args.Get(1).(int64), args.Error(2)
}
func (m *mockSKURepo) Update(ctx context.Context, item *domainSKU.SKU) error {
	return m.Called(ctx, item).Error(0)
}
func (m *mockSKURepo) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockSKURepo) ExistsBySKU(ctx context.Context, sku string) (bool, error) {
	args := m.Called(ctx, sku)
	return args.Bool(0), args.Error(1)
}
func (m *mockSKURepo) GetReceiptStatsBatch(ctx context.Context, skuIDs []uint) (map[uint]domainSKU.SKUBatchReceiptStat, error) {
	args := m.Called(ctx, skuIDs)
	return args.Get(0).(map[uint]domainSKU.SKUBatchReceiptStat), args.Error(1)
}
func (m *mockSKURepo) GetReceiptHistory(ctx context.Context, query domainSKU.SKUReceiptQuery) ([]domainSKU.SKUReceiptItem, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]domainSKU.SKUReceiptItem), args.Get(1).(int64), args.Error(2)
}

func TestCostHistoryResolution(t *testing.T) {
	ctx := context.Background()
	costRepo := new(mockCostHistoryRepo)
	skuRepo := new(mockSKURepo)

	uc := usecaseSKU.NewCostHistoryUsecase(costRepo, skuRepo)

	targetDate, _ := time.Parse("2006-01-02", "2026-04-15")

	// 1. Found in cost history
	costRepo.On("FindEffectiveCost", ctx, "SKU-001", targetDate).Return(45.50, true, nil).Once()

	cost, err := uc.GetEffectiveCost(ctx, "SKU-001", targetDate)
	assert.NoError(t, err)
	assert.Equal(t, 45.50, cost)

	// 2. Not in cost history, fallback to SKU Master cost
	costRepo.On("FindEffectiveCost", ctx, "SKU-002", targetDate).Return(0.0, false, nil).Once()
	skuRepo.On("FindBySKU", ctx, "SKU-002").Return(&domainSKU.SKU{
		SKU:       "SKU-002",
		CostPrice: 30.00,
	}, nil).Once()

	cost, err = uc.GetEffectiveCost(ctx, "SKU-002", targetDate)
	assert.NoError(t, err)
	assert.Equal(t, 30.00, cost)
}
