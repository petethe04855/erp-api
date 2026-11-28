package shopee_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
	usecaseShopee "chawy-erp-api/internal/usecase/shopee"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeInsightRepo struct{ lines []domainShopee.InsightLine }

func (f fakeInsightRepo) ListInsightLines(_ context.Context, until time.Time) ([]domainShopee.InsightLine, error) {
	var out []domainShopee.InsightLine
	for _, l := range f.lines {
		if l.OrderDate.Before(until) {
			out = append(out, l)
		}
	}
	return out, nil
}

func at(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 20, 0, 0, 0, bkk) }

// A hand-computed fixture (amounts in baht). Range: Jan–Mar 2026.
//
//	O0 d 2025-12-20 freeze-dried 1 bag 100       (before the range: d's first order)
//	O1 a 01-05 [โปร 3 ถุง] 300 + free box 0
//	O2 a 01-19 chicken "1 ถุง" ×2 = 200           (14 days after O1)
//	O3 b 02-10 fishing rod 59
//	O6 c 02-01 pickled ginger 100                (sister shop: left out)
//	O7 – 02-15 other product 40                  (no buyer name)
//	O8 d 02-20 chicken "3 ถุง" 250                (a repeat order of d, but d's only one in range)
//	O4 b 03-01 tuna SKU -05 = 5 bags 500          (19 days after O3)
//	O5 a 03-03 [โปร 3 ถุง] 300
func insightFixture() fakeInsightRepo {
	l := func(id, buyer string, d time.Time, product, variation, sku string, qty int, price float64) domainShopee.InsightLine {
		return domainShopee.InsightLine{OrderID: id, OrderDate: d, BuyerUsername: buyer, ProductName: product, VariationName: variation, SKU: sku, Qty: qty, SalePrice: price}
	}
	return fakeInsightRepo{lines: []domainShopee.InsightLine{
		l("O0", "d", at(2025, 12, 20), "อกไก่ฟรีซดราย", "1 ถุง", "", 1, 100),
		l("O1", "a", at(2026, 1, 5), "[โปร 3 ถุง] ฟรีซดราย", "อกไก่ 2 ทูน่า 1", "", 1, 300),
		l("O1", "a", at(2026, 1, 5), "กล่องของแถม", "", "", 1, 0),
		l("O2", "a", at(2026, 1, 19), "อกไก่ฟรีซดราย", "1 ถุง", "", 2, 200),
		l("O6", "c", at(2026, 2, 1), "ขิงดอง(แม่เพิ่ม) 2 ปอนด์", "", "K002", 1, 100),
		l("O3", "b", at(2026, 2, 10), "ไม้ตกแมว ขนนก", "", "", 1, 59),
		l("O7", "", at(2026, 2, 15), "สินค้าอื่น", "", "", 1, 40),
		l("O8", "d", at(2026, 2, 20), "อกไก่ฟรีซดราย", "3 ถุง", "", 1, 250),
		l("O4", "b", at(2026, 3, 1), "ทูน่าฟรีซดราย", "", "FD-TN-05", 1, 500),
		l("O5", "a", at(2026, 3, 3), "[โปร 3 ถุง] ฟรีซดราย", "", "", 1, 300),
	}}
}

func insightRange(t *testing.T) usecaseShopee.InsightRange {
	rng, err := usecaseShopee.ParseInsightRange("2026-01", "2026-03", time.Now())
	require.NoError(t, err)
	return rng
}

func TestParseInsightRange(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, bkk)
	rng, err := usecaseShopee.ParseInsightRange("", "", now)
	require.NoError(t, err)
	assert.Equal(t, "2026-01", rng.From)
	assert.Equal(t, "2026-11", rng.To)
	assert.Len(t, rng.Months, 11)
	assert.True(t, rng.End.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, bkk)))

	for _, bad := range [][2]string{{"2026-13", "2026-12"}, {"2026-05", "2026-04"}, {"2024-01", "2026-02"}, {"26-01", ""}} {
		_, err := usecaseShopee.ParseInsightRange(bad[0], bad[1], now)
		assert.ErrorIs(t, err, usecaseShopee.ErrInvalidInsightRange, bad)
	}
}

func TestInsightSummary(t *testing.T) {
	uc := usecaseShopee.NewInsightUsecase(insightFixture())
	s, err := uc.Summary(context.Background(), insightRange(t))
	require.NoError(t, err)

	assert.Equal(t, 7, s.Meta.OrdersScanned)
	assert.Equal(t, 1, s.Meta.ExcludedOrders, "the pickled-ginger order is not Chawy's")
	assert.Equal(t, 1, s.Meta.UnnamedOrders)

	k := s.KPI
	assert.Equal(t, 1649.0, k.TotalSales)
	assert.Equal(t, 7, k.Orders)
	assert.Equal(t, 3, k.Customers)
	assert.Equal(t, 235.57, k.AOV)
	assert.Equal(t, 2, k.RepeatCust)
	assert.Equal(t, 66.67, k.RepeatPct)
	assert.Equal(t, 82.41, k.RepeatSalesPct)
	require.NotNil(t, k.MedianGapDays)
	assert.Equal(t, 16.5, *k.MedianGapDays)
	assert.Equal(t, "2026-03", k.LastMonth)
	assert.Equal(t, "2026-02", k.PrevMonth)
	assert.Equal(t, 800.0, k.LastSales)
	assert.Equal(t, 349.0, k.PrevSales)
	require.NotNil(t, k.MoMPct)
	assert.Equal(t, 129.23, *k.MoMPct)

	assert.Equal(t, []usecaseShopee.InsightMonth{
		{Month: "2026-01", Sales: 500, Orders: 2, NewCustomers: 1, RepeatOrders: 1},
		{Month: "2026-02", Sales: 349, Orders: 3, NewCustomers: 1, RepeatOrders: 1},
		{Month: "2026-03", Sales: 800, Orders: 2, NewCustomers: 0, RepeatOrders: 2},
	}, s.Monthly)
}

func TestInsightProducts(t *testing.T) {
	uc := usecaseShopee.NewInsightUsecase(insightFixture())
	p, err := uc.Products(context.Background(), insightRange(t))
	require.NoError(t, err)

	names := []string{}
	for _, r := range p.Products {
		names = append(names, r.Product)
	}
	assert.Equal(t, []string{"[โปร 3 ถุง] ฟรีซดราย", "ทูน่าฟรีซดราย", "อกไก่ฟรีซดราย", "ไม้ตกแมว ขนนก", "สินค้าอื่น"}, names,
		"sorted by sales; the free box is not a product sale")
	assert.Equal(t, usecaseShopee.InsightProduct{Category: usecaseShopee.CategoryFDSolo, Product: "อกไก่ฟรีซดราย", Orders: 2, Qty: 3, Sales: 450}, p.Products[2])
	assert.Equal(t, "FD-TN-05", p.Products[1].SKU)

	cat := map[string]float64{}
	for _, r := range p.MonthlyCategory {
		cat[r.Month+" "+r.Category] = r.Sales
	}
	assert.Len(t, p.MonthlyCategory, 12, "every month × category, zeros included")
	assert.Equal(t, 300.0, cat["2026-01 "+usecaseShopee.CategoryFDMix])
	assert.Equal(t, 200.0, cat["2026-01 "+usecaseShopee.CategoryFDSolo])
	assert.Equal(t, 59.0, cat["2026-02 "+usecaseShopee.CategoryCatToy])
	assert.Equal(t, 500.0, cat["2026-03 "+usecaseShopee.CategoryFDSolo])
}

func TestInsightSets(t *testing.T) {
	uc := usecaseShopee.NewInsightUsecase(insightFixture())
	s, err := uc.Sets(context.Background(), insightRange(t))
	require.NoError(t, err)

	assert.Equal(t, []int{1, 3, 5}, s.SetSizes)
	assert.Equal(t, 0, s.UnresolvedLines)
	got := map[string]usecaseShopee.InsightMonthSet{}
	for _, r := range s.MonthlySets {
		got[r.Month+"/"+string(rune('0'+r.Set))] = r
	}
	assert.Len(t, s.MonthlySets, 9)
	assert.Equal(t, 2, got["2026-01/1"].SetsSold)
	assert.Equal(t, 200.0, got["2026-01/1"].Sales)
	assert.Equal(t, 1, got["2026-02/3"].SetsSold)
	assert.Equal(t, 500.0, got["2026-03/5"].Sales)

	bags := map[string]usecaseShopee.InsightBagsBucket{}
	for _, b := range s.BagsPerOrder {
		bags[b.Bags] = b
	}
	assert.Equal(t, 1, bags["2"].Orders)
	assert.Equal(t, 3, bags["3"].Orders)
	assert.Equal(t, 850.0, bags["3"].Sales)
	assert.Equal(t, 1, bags["5"].Orders)
	assert.Equal(t, 0, bags["11+"].Orders)
}

func TestInsightCustomers(t *testing.T) {
	uc := usecaseShopee.NewInsightUsecase(insightFixture())
	c, err := uc.Customers(context.Background(), insightRange(t))
	require.NoError(t, err)

	assert.Equal(t, []usecaseShopee.InsightFrequency{
		{Purchases: 1, Customers: 1, Sales: 250},
		{Purchases: 2, Customers: 1, Sales: 559},
		{Purchases: 3, Customers: 1, Sales: 800},
	}, c.Frequency)
	require.NotNil(t, c.RepeatGap.MedianDays)
	assert.Equal(t, 16.5, *c.RepeatGap.MedianDays)
	assert.Len(t, c.RepeatGap.Weeks, 21)
	assert.Equal(t, "21+", c.RepeatGap.Weeks[20].Week)
	assert.Equal(t, 2, c.RepeatGap.Weeks[2].Customers, "14 and 19 days are both week 3")
	assert.Equal(t, 0.0, c.RepeatGap.Weeks[1].CumPct)
	assert.Equal(t, 100.0, c.RepeatGap.Weeks[2].CumPct)
}

func TestInsightJourney(t *testing.T) {
	uc := usecaseShopee.NewInsightUsecase(insightFixture())
	j, err := uc.Journey(context.Background(), insightRange(t), 4)
	require.NoError(t, err)

	assert.Equal(t, []string{"ฟรีซดราย 1 ถุง", "ฟรีซดราย 3 ถุง", "ฟรีซดราย 5 ถุง", usecaseShopee.MainFishingRod, usecaseShopee.JourneyStopNode}, j.Nodes)
	assert.ElementsMatch(t, []usecaseShopee.InsightJourneyLink{
		{FromOrder: 1, From: "ฟรีซดราย 3 ถุง", ToOrder: 2, To: "ฟรีซดราย 1 ถุง", Customers: 1},
		{FromOrder: 1, From: "ฟรีซดราย 3 ถุง", ToOrder: 2, To: usecaseShopee.JourneyStopNode, Customers: 1}, // d
		{FromOrder: 1, From: usecaseShopee.MainFishingRod, ToOrder: 2, To: "ฟรีซดราย 5 ถุง", Customers: 1},
		{FromOrder: 2, From: "ฟรีซดราย 1 ถุง", ToOrder: 3, To: "ฟรีซดราย 3 ถุง", Customers: 1},
		{FromOrder: 2, From: "ฟรีซดราย 5 ถุง", ToOrder: 3, To: usecaseShopee.JourneyStopNode, Customers: 1},
		{FromOrder: 3, From: "ฟรีซดราย 3 ถุง", ToOrder: 4, To: usecaseShopee.JourneyStopNode, Customers: 1},
	}, j.Links)

	_, err = uc.Journey(context.Background(), insightRange(t), 9)
	assert.Error(t, err)
}

// No insight response may carry a buyer's username.
func TestInsightResponsesHaveNoBuyerNames(t *testing.T) {
	uc := usecaseShopee.NewInsightUsecase(insightFixture())
	ctx := context.Background()
	rng := insightRange(t)
	s, _ := uc.Summary(ctx, rng)
	p, _ := uc.Products(ctx, rng)
	st, _ := uc.Sets(ctx, rng)
	c, _ := uc.Customers(ctx, rng)
	j, _ := uc.Journey(ctx, rng, 4)
	for _, v := range []any{s, p, st, c, j} {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		body := string(b)
		assert.NotContains(t, body, "buyer")
		for _, name := range []string{`"a"`, `"b"`, `"d"`} {
			assert.False(t, strings.Contains(body, name), "buyer %s leaked in %s", name, body)
		}
	}
}
