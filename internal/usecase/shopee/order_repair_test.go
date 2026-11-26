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

const repairHeader = "หมายเลขคำสั่งซื้อ,ชื่อผู้ใช้ (ผู้ซื้อ),วันที่ทำการสั่งซื้อ,เลขอ้างอิง SKU (SKU Reference No.),ชื่อสินค้า,ชื่อตัวเลือก,ราคาขาย,จำนวน,ราคาขายสุทธิ,จำนวนเงินทั้งหมด,จังหวัด,เวลาที่ทำการสั่งซื้อสำเร็จ\n"

var bkk = time.FixedZone("ICT", 7*60*60)

// storedByOldParser is how the old parser saved order A: completion time read
// as UTC, quantity 1, no variation. Line 11 has a SKU the user corrected.
func storedByOldParser() []domainShopee.ShopeeOrder {
	return []domainShopee.ShopeeOrder{
		{
			ID:        "A",
			OrderDate: time.Date(2026, 7, 2, 10, 15, 0, 0, time.UTC),
			Items: []domainShopee.ShopeeOrderItem{
				{ID: 10, OrderID: "A", SKU: "FD-03", OriginalSKU: "FD-03", ProductName: "ฟรีซดราย", Qty: 1, SalePrice: 918},
				{ID: 11, OrderID: "A", SKU: "TOY-FIXED", OriginalSKU: "TOY-1", ProductName: "ไม้ตกแมว", Qty: 1, SalePrice: 59, SKUConfirmed: true},
			},
		},
		{
			ID:        "B",
			OrderDate: time.Date(2026, 6, 5, 9, 0, 0, 0, bkk),
			Items: []domainShopee.ShopeeOrderItem{
				{ID: 20, OrderID: "B", SKU: "FD-01", OriginalSKU: "FD-01", Qty: 1, SalePrice: 159, VariationName: "อกไก่"},
			},
		},
		{
			ID:        "C",
			OrderDate: time.Date(2026, 6, 6, 9, 0, 0, 0, bkk),
			Items: []domainShopee.ShopeeOrderItem{
				{ID: 30, OrderID: "C", SKU: "X", OriginalSKU: "X", Qty: 1, SalePrice: 100},
			},
		},
	}
}

const repairFile = repairHeader +
	// A: ordered 30 Jun 21:09 (Bangkok), completed 2 Jul; 2 sets of 3 bags.
	"A,u1,2026-06-30 21:09,FD-03,ฟรีซดราย,3 ถุง,459.00,2,918.00,735.00,กทม,2026-07-02 17:15\n" +
	"A,u1,2026-06-30 21:09,TOY-1,ไม้ตกแมว,,59.00,1,59.00,47.00,กทม,2026-07-02 17:15\n" +
	// B: already correct.
	"B,u2,2026-06-05 09:00,FD-01,ฟรีซดราย,อกไก่,159.00,1,159.00,127.00,กทม,2026-06-08 10:00\n" +
	// C: the file has a line the stored order does not.
	"C,u3,2026-06-06 09:00,X,สินค้า,,100.00,1,100.00,80.00,กทม,2026-06-09 10:00\n" +
	"C,u3,2026-06-06 09:00,Y,สินค้า,,50.00,1,50.00,40.00,กทม,2026-06-09 10:00\n" +
	// D: not imported yet; a repair does not insert it.
	"D,u4,2026-06-07 09:00,Z,สินค้า,,10.00,1,10.00,8.00,กทม,2026-06-09 10:00\n"

func TestOrderRepair_PreviewFindsWhatTheOldParserGotWrong(t *testing.T) {
	ctx := context.Background()
	repo := new(mockShopeeOrderRepo)
	repo.On("FindByOrderIDs", ctx, mock.Anything).Return(storedByOldParser(), nil)
	uc := usecaseShopee.NewOrderUsecase(repo)

	p, err := uc.PreviewOrderRepair(ctx, strings.NewReader(repairFile), "Order.completed.csv")
	assert.NoError(t, err)
	assert.Equal(t, 4, p.OrdersInFile)
	assert.Equal(t, 3, p.OrdersInDB)
	assert.Equal(t, 1, p.OrdersNotInDB)
	assert.Equal(t, 1, p.OrdersUnchanged, "B is already right")
	assert.Equal(t, 1, p.OrdersToRepair)
	assert.Equal(t, 1, p.DateChanges)
	assert.Equal(t, 1, p.MonthChanges, "A moves from July back to June")
	assert.Equal(t, 1, p.QtyChanges)
	assert.Equal(t, 1, p.VariationChanges)
	if assert.Len(t, p.Skipped, 1) {
		assert.Equal(t, "C", p.Skipped[0].OrderID, "lines that do not pair up are skipped, not guessed")
	}

	if !assert.Len(t, p.Repairs, 1) {
		return
	}
	a := p.Repairs[0]
	assert.True(t, a.DateChanged)
	assert.True(t, a.NewOrderDate.Equal(time.Date(2026, 6, 30, 21, 9, 0, 0, bkk)))
	if assert.Len(t, a.Items, 1, "the toy line is unchanged even though its SKU was corrected") {
		it := a.Items[0]
		assert.Equal(t, uint(10), it.ItemID)
		assert.Equal(t, 1, it.OldQty)
		assert.Equal(t, 2, it.NewQty)
		assert.Equal(t, "3 ถุง", it.NewVariation)
	}
	repo.AssertNotCalled(t, "ApplyRepairs", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestOrderRepair_ApplySendsOnlyThePlannedChanges(t *testing.T) {
	ctx := context.Background()
	repo := new(mockShopeeOrderRepo)
	repo.On("FindByOrderIDs", ctx, mock.Anything).Return(storedByOldParser(), nil)
	repo.On("ApplyRepairs", ctx, mock.MatchedBy(func(r []domainShopee.OrderRepair) bool {
		return len(r) == 1 && r[0].OrderID == "A" && len(r[0].Items) == 1 && r[0].Items[0].ItemID == 10
	}), mock.MatchedBy(func(batch string) bool { return strings.HasPrefix(batch, "repair-") }),
		"owner@chawy.test", "Order.completed.csv").Return(nil).Once()
	uc := usecaseShopee.NewOrderUsecase(repo)

	res, err := uc.ApplyOrderRepair(ctx, strings.NewReader(repairFile), "Order.completed.csv", "owner@chawy.test")
	assert.NoError(t, err)
	assert.Equal(t, 1, res.OrdersRepaired)
	assert.Equal(t, 1, res.ItemsRepaired)
	assert.Len(t, res.Skipped, 1)
	repo.AssertExpectations(t)
}

func TestOrderRepair_KeepsStoredVariationWhenFileHasNone(t *testing.T) {
	ctx := context.Background()
	repo := new(mockShopeeOrderRepo)
	repo.On("FindByOrderIDs", ctx, mock.Anything).Return([]domainShopee.ShopeeOrder{{
		ID:        "E",
		OrderDate: time.Date(2026, 6, 5, 9, 0, 0, 0, bkk),
		Items:     []domainShopee.ShopeeOrderItem{{ID: 1, SKU: "S", OriginalSKU: "S", Qty: 1, SalePrice: 10, VariationName: "อกไก่"}},
	}}, nil)
	uc := usecaseShopee.NewOrderUsecase(repo)

	file := repairHeader + "E,u,2026-06-05 09:00,S,สินค้า,,10.00,1,10.00,8.00,กทม,2026-06-08 10:00\n"
	p, err := uc.PreviewOrderRepair(ctx, strings.NewReader(file), "x.csv")
	assert.NoError(t, err)
	assert.Equal(t, 0, p.OrdersToRepair)
	assert.Equal(t, 1, p.OrdersUnchanged)
}
