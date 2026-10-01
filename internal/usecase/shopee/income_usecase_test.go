package shopee_test

import (
	"context"
	"strings"
	"testing"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
	usecaseShopee "chawy-erp-api/internal/usecase/shopee"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockShopeeIncomeRepo struct {
	mock.Mock
}

func (m *mockShopeeIncomeRepo) BulkInsert(ctx context.Context, incomes []domainShopee.ShopeeIncome) (int, int, error) {
	args := m.Called(ctx, incomes)
	return args.Int(0), args.Int(1), args.Error(2)
}

func (m *mockShopeeIncomeRepo) FindAll(ctx context.Context, filter domainShopee.IncomeFilter) ([]domainShopee.ShopeeIncome, int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domainShopee.ShopeeIncome), args.Get(1).(int64), args.Error(2)
}

func (m *mockShopeeIncomeRepo) FindByMonth(ctx context.Context, year int, month int) ([]domainShopee.ShopeeIncome, error) {
	args := m.Called(ctx, year, month)
	return args.Get(0).([]domainShopee.ShopeeIncome), args.Error(1)
}

func (m *mockShopeeIncomeRepo) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockShopeeIncomeRepo) GetMatchedOrderIDs(ctx context.Context, orderIDs []string) (map[string]bool, error) {
	args := m.Called(ctx, orderIDs)
	return args.Get(0).(map[string]bool), args.Error(1)
}

func TestIncomeUsecase_PreviewAndImport(t *testing.T) {
	ctx := context.Background()
	incomeRepo := new(mockShopeeIncomeRepo)
	uc := usecaseShopee.NewIncomeUsecase(incomeRepo)

	csvContent := `หมายเลขคำสั่งซื้อ,วันที่ทำการสั่งซื้อ,วันที่โอนชำระเงินสำเร็จ,จำนวนเงินทั้งหมดที่โอนแล้ว (฿)
260301ABC01,2026-03-01 10:30:00,2026-03-05 16:00:00,135.50
260301ABC02,2026-03-02 14:15:00,2026-03-06 17:30:00,180.00
`

	// 1. Preview
	preview, err := uc.PreviewIncomeFile(ctx, strings.NewReader(csvContent), "income.csv")
	assert.NoError(t, err)
	assert.Equal(t, 2, preview.TotalRows)
	assert.Equal(t, 315.50, preview.TotalNetAmount)
	assert.Equal(t, "260301ABC01", preview.SampleRows[0].OrderID)
	assert.Equal(t, 135.50, preview.SampleRows[0].NetAmount)

	// Verify Transfer Date parsed
	expectedTransfer, _ := time.Parse("2006-01-02 15:04:05", "2026-03-05 16:00:00")
	assert.Equal(t, expectedTransfer, preview.SampleRows[0].TransferDate)

	// 2. Import
	incomeRepo.On("BulkInsert", ctx, mock.MatchedBy(func(incomes []domainShopee.ShopeeIncome) bool {
		return len(incomes) == 2
	})).Return(2, 0, nil).Once()

	importRes, err := uc.ImportIncomeFile(ctx, strings.NewReader(csvContent), "income.csv")
	assert.NoError(t, err)
	assert.Equal(t, 2, importRes.InsertedCount)
	assert.Equal(t, 315.50, importRes.TotalAmount)
}
