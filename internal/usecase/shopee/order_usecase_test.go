package shopee_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

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

func (m *mockShopeeOrderRepo) ApplyRepairs(ctx context.Context, repairs []domainShopee.OrderRepair, batchID, actor, sourceFile string) error {
	args := m.Called(ctx, repairs, batchID, actor, sourceFile)
	return args.Error(0)
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

// The real Seller Centre "Order.completed" export has several headers that
// contain the words of another field ("จำนวนเงินทั้งหมด" holds "จำนวน",
// "เวลาที่ทำการสั่งซื้อสำเร็จ" holds the order-time words, "ราคาขาย" comes
// before "ราคาขายสุทธิ"). Each field must still come from its own column.
func TestOrderUsecase_SellerCentreExportColumns(t *testing.T) {
	ctx := context.Background()
	orderRepo := new(mockShopeeOrderRepo)
	uc := usecaseShopee.NewOrderUsecase(orderRepo)
	orderRepo.On("FindByOrderIDs", ctx, mock.Anything).Return([]domainShopee.ShopeeOrder{}, nil)

	csvContent := `หมายเลขคำสั่งซื้อ,ชื่อผู้ใช้ (ผู้ซื้อ),วันที่ทำการสั่งซื้อ,ตัวเลือกการจัดส่ง,เลขอ้างอิง Parent SKU,ชื่อสินค้า,เลขอ้างอิง SKU (SKU Reference No.),ชื่อตัวเลือก,ราคาตั้งต้น,ราคาขาย,จำนวน,จำนวนที่ส่งคืน,ราคาขายสุทธิ,จำนวนเงินทั้งหมด,จังหวัด,เวลาที่ทำการสั่งซื้อสำเร็จ
2606300ABC,buyer_a,2026-06-30 21:09,Standard Delivery,FD-MX,[โปร 5 ถุง] ฟรีซดราย,FD-MX-C1T2R2-05,ไก่ 1 ทูน่า 2 ตับ 2,500.00,459.00,2,0,918.00,735.00,จังหวัดเชียงใหม่,2026-07-04 10:15
`

	preview, err := uc.PreviewOrderFile(ctx, strings.NewReader(csvContent), "Order.completed.csv")
	assert.NoError(t, err)
	if !assert.Len(t, preview.SampleRows, 1) || !assert.Len(t, preview.SampleOrders, 1) {
		return
	}

	item := preview.SampleRows[0]
	assert.Equal(t, 2, item.Qty, "qty comes from จำนวน, not จำนวนเงินทั้งหมด")
	assert.Equal(t, 918.00, item.SalePrice, "sale price is ราคาขายสุทธิ (line total)")
	assert.Equal(t, "FD-MX-C1T2R2-05", item.SKU, "SKU reference wins over parent SKU")
	assert.Equal(t, "ไก่ 1 ทูน่า 2 ตับ 2", item.VariationName)

	order := preview.SampleOrders[0]
	bangkok := time.FixedZone("ICT", 7*60*60)
	assert.True(t, order.OrderDate.Equal(time.Date(2026, 6, 30, 21, 9, 0, 0, bangkok)),
		"order date is วันที่ทำการสั่งซื้อ read as Bangkok time, got %s", order.OrderDate)
	assert.Equal(t, "buyer_a", order.BuyerUsername)
	assert.Equal(t, "จังหวัดเชียงใหม่", order.Province)
}

func TestOrderUsecase_QtyFormattedAsDecimal(t *testing.T) {
	ctx := context.Background()
	orderRepo := new(mockShopeeOrderRepo)
	uc := usecaseShopee.NewOrderUsecase(orderRepo)
	orderRepo.On("FindByOrderIDs", ctx, mock.Anything).Return([]domainShopee.ShopeeOrder{}, nil)

	csvContent := `หมายเลขคำสั่งซื้อ,วันที่ทำการสั่งซื้อ,เลขอ้างอิง SKU,ชื่อสินค้า,จำนวน,ราคาขายสุทธิ
260301QTY01,2026-03-01 10:30,SKU-A01,สินค้า,3.00,300.00
260301QTY02,2026-03-01 10:30,SKU-A02,สินค้า,,100.00
`
	preview, err := uc.PreviewOrderFile(ctx, strings.NewReader(csvContent), "orders.csv")
	assert.NoError(t, err)
	if assert.Len(t, preview.SampleRows, 2) {
		assert.Equal(t, 3, preview.SampleRows[0].Qty)
		assert.Equal(t, 1, preview.SampleRows[1].Qty, "a blank quantity still counts as one")
	}
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

func TestOrderUsecase_ImportSkipsDuplicateRows(t *testing.T) {
	ctx := context.Background()
	orderRepo := new(mockShopeeOrderRepo)
	uc := usecaseShopee.NewOrderUsecase(orderRepo)

	csvContent := `หมายเลขคำสั่งซื้อ,วันที่ทำการสั่งซื้อ,เลขอ้างอิง SKU,ชื่อสินค้า,จำนวน,ราคาขาย,ชื่อผู้ใช้ (ผู้ซื้อ),จังหวัด
260301DUP01,2026-03-01 10:30:00,SKU-A01,สินค้าทดสอบ,1,100.00,user1,กทม
260301DUP01,2026-03-01 10:30:00,SKU-A01,สินค้าทดสอบ,1,100.00,user1,กทม
260301EXISTING,2026-03-02 10:30:00,SKU-B01,สินค้ามีในระบบแล้ว,1,150.00,user2,เชียงใหม่
260301NEW01,2026-03-03 10:30:00,SKU-C01,สินค้าใหม่,1,80.00,user3,ภูเก็ต
`

	orderRepo.On("FindByOrderIDs", ctx, mock.Anything).Return([]domainShopee.ShopeeOrder{{ID: "260301EXISTING"}}, nil)
	// The in-file duplicate row is dropped before insert; the DB-existing order is
	// still passed so BulkInsert can skip it, and the rest is imported normally.
	orderRepo.On("BulkInsert", ctx, mock.MatchedBy(func(orders []domainShopee.ShopeeOrder) bool {
		if len(orders) != 3 {
			return false
		}
		for _, o := range orders {
			if o.ID == "260301DUP01" && len(o.Items) != 1 {
				return false
			}
		}
		return true
	})).Return(2, 1, nil).Once()

	res, err := uc.ImportOrderFile(ctx, strings.NewReader(csvContent), "orders.csv")
	assert.NoError(t, err)
	assert.Equal(t, 2, res.InsertedCount)
	assert.Equal(t, 1, res.SkippedCount)
	assert.True(t, res.DuplicateCount >= 2)
	orderRepo.AssertExpectations(t)
}
