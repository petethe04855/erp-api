package shopee_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	domainShopee "chawy-erp-api/internal/domain/shopee"
	usecaseShopee "chawy-erp-api/internal/usecase/shopee"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockShopeeOrderRepo struct {
	mock.Mock
}

func (m *mockShopeeOrderRepo) BulkInsert(ctx context.Context, orders []domainShopee.ShopeeOrder) (int, int, error) {
	args := m.Called(ctx, orders)
	return args.Int(0), args.Int(1), args.Error(2)
}

func (m *mockShopeeOrderRepo) FindAll(ctx context.Context, filter domainShopee.OrderFilter) ([]domainShopee.ShopeeOrder, int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domainShopee.ShopeeOrder), args.Get(1).(int64), args.Error(2)
}

func (m *mockShopeeOrderRepo) FindByID(ctx context.Context, id string) (*domainShopee.ShopeeOrder, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainShopee.ShopeeOrder), args.Error(1)
}

func (m *mockShopeeOrderRepo) UpdateItemSKU(ctx context.Context, itemID uint, newSKU string, confirmed bool) error {
	args := m.Called(ctx, itemID, newSKU, confirmed)
	return args.Error(0)
}

func (m *mockShopeeOrderRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockShopeeOrderRepo) GetDistinctProvinces(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockShopeeOrderRepo) FindByOrderIDs(ctx context.Context, orderIDs []string) ([]domainShopee.ShopeeOrder, error) {
	args := m.Called(ctx, orderIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domainShopee.ShopeeOrder), args.Error(1)
}

func TestOrderUsecase_PreviewAndImport(t *testing.T) {
	ctx := context.Background()
	orderRepo := new(mockShopeeOrderRepo)
	uc := usecaseShopee.NewOrderUsecase(orderRepo)

	csvContent := `หมายเลขคำสั่งซื้อ,วันที่ทำการสั่งซื้อ,เลขอ้างอิง SKU,ชื่อสินค้า,จำนวน,ราคาขาย,ชื่อผู้ใช้ (ผู้ซื้อ),จังหวัด
260301ABC01,2026-03-01 10:30:00,SKU-A01,เสื้อผ้าทดสอบ,2,100.00,user_buyer_1,กรุงเทพมหานคร
260301ABC01,2026-03-01 10:30:00,SKU-A02,กางเกงทดสอบ,1,50.00,user_buyer_1,กรุงเทพมหานคร
260301ABC02,2026-03-02 14:15:00,,สินค้าไม่มีSKU,1,200.00,user_buyer_2,เชียงใหม่
`

	orderRepo.On("FindByOrderIDs", ctx, mock.Anything).Return([]domainShopee.ShopeeOrder{}, nil)

	// 1. Test Preview
	preview, err := uc.PreviewOrderFile(ctx, strings.NewReader(csvContent), "orders.csv")
	assert.NoError(t, err)
	assert.Equal(t, 3, preview.TotalRows)
	assert.Equal(t, 2, preview.TotalOrders)
	assert.Equal(t, 1, preview.BlankSKUCount)
	assert.Equal(t, 3, len(preview.SampleRows))

	// Verify BR-01: sale_price is preserved as line total
	assert.Equal(t, 100.00, preview.SampleRows[0].SalePrice)
	assert.Equal(t, 2, preview.SampleRows[0].Qty)

	// 2. Test Import
	orderRepo.On("BulkInsert", ctx, mock.MatchedBy(func(orders []domainShopee.ShopeeOrder) bool {
		return len(orders) == 2
	})).Return(2, 0, nil).Once()

	importRes, err := uc.ImportOrderFile(ctx, bytes.NewBufferString(csvContent), "orders.csv")
	assert.NoError(t, err)
	assert.Equal(t, 2, importRes.InsertedCount)
	assert.Equal(t, 3, importRes.TotalRows)
}

func TestOrderUsecase_DuplicateDetection(t *testing.T) {
	ctx := context.Background()
	orderRepo := new(mockShopeeOrderRepo)
	uc := usecaseShopee.NewOrderUsecase(orderRepo)

	// CSV containing internal duplicate row and existing DB order
	csvContent := `หมายเลขคำสั่งซื้อ,วันที่ทำการสั่งซื้อ,เลขอ้างอิง SKU,ชื่อสินค้า,จำนวน,ราคาขาย,ชื่อผู้ใช้ (ผู้ซื้อ),จังหวัด
260301DUP01,2026-03-01 10:30:00,SKU-A01,สินค้าทดสอบ,1,100.00,user1,กทม
260301DUP01,2026-03-01 10:30:00,SKU-A01,สินค้าทดสอบ,1,100.00,user1,กทม
260301EXISTING,2026-03-02 10:30:00,SKU-B01,สินค้ามีในระบบแล้ว,1,150.00,user2,เชียงใหม่
`

	orderRepo.On("FindByOrderIDs", ctx, mock.Anything).Return([]domainShopee.ShopeeOrder{
		{ID: "260301EXISTING"},
	}, nil)

	preview, err := uc.PreviewOrderFile(ctx, strings.NewReader(csvContent), "orders.csv")
	assert.NoError(t, err)
	assert.True(t, preview.DuplicateCount >= 2)
	assert.Contains(t, preview.DuplicateOrders, "260301DUP01")
	assert.Contains(t, preview.DuplicateOrders, "260301EXISTING")
}
