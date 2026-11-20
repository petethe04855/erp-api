package salesanalytics

import (
	"context"
	"testing"

	domainFormula "chawy-erp-api/internal/domain/formula"
	"chawy-erp-api/internal/domain/salesanalytics"
	domainSKU "chawy-erp-api/internal/domain/sku"

	"github.com/stretchr/testify/require"
)

type fakeFormulas struct {
	byCode map[string]*domainFormula.InventoryFormula
	calls  int
}

func (f *fakeFormulas) FindByCode(_ context.Context, code string) (*domainFormula.InventoryFormula, error) {
	f.calls++
	return f.byCode[code], nil
}

type fakeSKUs struct {
	byCode map[string]*domainSKU.SKU
	calls  int
}

func (f *fakeSKUs) FindBySKU(_ context.Context, code string) (*domainSKU.SKU, error) {
	f.calls++
	return f.byCode[code], nil
}

func TestSalesAnalyticsCountsBagsPerFlavor(t *testing.T) {
	formulas := &fakeFormulas{byCode: map[string]*domainFormula.InventoryFormula{
		// A 3-bag mixed set and a 2-bag chicken set.
		"SET-MIX": {Code: "SET-MIX", IsActive: true, Items: []domainFormula.InventoryFormulaItem{
			{ComponentSKU: "chk-01", Qty: 1}, {ComponentSKU: "TUN-01", Qty: 1}, {ComponentSKU: "RAB-01", Qty: 1},
		}},
		"SET-CHK2": {Code: "SET-CHK2", IsActive: true, Items: []domainFormula.InventoryFormulaItem{{ComponentSKU: "CHK-01", Qty: 2}}},
		// Inactive formulas are ignored: the SKU counts as itself.
		"OLD-SET": {Code: "OLD-SET", IsActive: false, Items: []domainFormula.InventoryFormulaItem{{ComponentSKU: "CHK-01", Qty: 5}}},
	}}
	skus := &fakeSKUs{byCode: map[string]*domainSKU.SKU{
		"CHK-01":  {SKU: "CHK-01", Flavor: "chicken"},
		"TUN-01":  {SKU: "TUN-01", Flavor: "tuna"},
		"RAB-01":  {SKU: "RAB-01", Flavor: "Rabbit"},
		"TOY-01":  {SKU: "TOY-01"}, // accessory: no flavor
		"OLD-SET": {SKU: "OLD-SET"},
	}}
	repo := &fakeRepo{orders: []salesanalytics.Order{
		{OrderNo: "A", Lines: []salesanalytics.Line{
			{SKU: " set-mix ", Qty: 2}, // 2 chicken, 2 tuna, 2 rabbit
			{SKU: "TUN-01", Qty: 1},    // direct single bag
			{SKU: "TOY-01", Qty: 3},    // not a bag
		}},
		{OrderNo: "B", Lines: []salesanalytics.Line{{SKU: "SET-CHK2", Qty: 1}, {SKU: "OLD-SET", Qty: 1}, {SKU: "UNKNOWN", Qty: 4}}},
	}}
	uc := NewUsecase(repo, WithFlavorSources(formulas, skus))

	out, err := uc.SalesAnalytics(context.Background(), salesanalytics.Query{DateFrom: "2026-11-01", DateTo: "2026-11-30"})
	require.NoError(t, err)

	a := out.Orders[0]
	require.Equal(t, 2, a.ChickenBags)
	require.Equal(t, 3, a.TunaBags)
	require.Equal(t, 2, a.RabbitBags)
	require.Equal(t, 7, a.TotalBags)
	require.Equal(t, MixedFlavor, a.Lines[0].Flavor)
	require.Equal(t, "tuna", a.Lines[1].Flavor)
	require.Equal(t, "", a.Lines[2].Flavor, "accessories have no flavor")

	b := out.Orders[1]
	require.Equal(t, 2, b.ChickenBags)
	require.Equal(t, 2, b.TotalBags, "inactive formula and unknown SKUs add no bags")
	require.Equal(t, "chicken", b.Lines[0].Flavor)

	require.LessOrEqual(t, skus.calls, 6, "each SKU is looked up once per request")
}

func TestSalesAnalyticsWithoutFlavorSourcesLeavesBagsEmpty(t *testing.T) {
	repo := &fakeRepo{orders: []salesanalytics.Order{{OrderNo: "A", Lines: []salesanalytics.Line{{SKU: "CHK-01", Qty: 1}}}}}
	out, err := NewUsecase(repo).SalesAnalytics(context.Background(), salesanalytics.Query{DateFrom: "2026-11-01", DateTo: "2026-11-30"})
	require.NoError(t, err)
	require.Zero(t, out.Orders[0].TotalBags)
}
