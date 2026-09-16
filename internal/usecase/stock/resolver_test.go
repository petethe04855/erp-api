package stock_test

import (
	"context"
	"testing"

	domainFormula "chawy-erp-api/internal/domain/formula"
	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseStock "chawy-erp-api/internal/usecase/stock"
)

type mockFormulaRepo struct {
	formulas map[string]*domainFormula.InventoryFormula
}

func (m *mockFormulaRepo) Create(ctx context.Context, f *domainFormula.InventoryFormula) error {
	m.formulas[f.Code] = f
	return nil
}
func (m *mockFormulaRepo) FindByID(ctx context.Context, id uint) (*domainFormula.InventoryFormula, error) {
	for _, f := range m.formulas {
		if f.ID == id {
			return f, nil
		}
	}
	return nil, nil
}
func (m *mockFormulaRepo) FindByCode(ctx context.Context, code string) (*domainFormula.InventoryFormula, error) {
	if f, ok := m.formulas[code]; ok {
		return f, nil
	}
	return nil, nil
}
func (m *mockFormulaRepo) Update(ctx context.Context, f *domainFormula.InventoryFormula) error {
	m.formulas[f.Code] = f
	return nil
}
func (m *mockFormulaRepo) Delete(ctx context.Context, code string) error {
	delete(m.formulas, code)
	return nil
}
func (m *mockFormulaRepo) Deactivate(ctx context.Context, code string) error {
	if f, ok := m.formulas[code]; ok {
		f.IsActive = false
	}
	return nil
}
func (m *mockFormulaRepo) ToggleStatus(ctx context.Context, code string, isActive bool) error {
	if f, ok := m.formulas[code]; ok {
		f.IsActive = isActive
	}
	return nil
}
func (m *mockFormulaRepo) FindAll(ctx context.Context, q domainFormula.Query) ([]domainFormula.InventoryFormula, int64, error) {
	var list []domainFormula.InventoryFormula
	for _, f := range m.formulas {
		list = append(list, *f)
	}
	return list, int64(len(list)), nil
}
func (m *mockFormulaRepo) ExistsByCode(ctx context.Context, code string) (bool, error) {
	_, ok := m.formulas[code]
	return ok, nil
}

type mockSKURepo struct {
	skus map[string]*domainSKU.SKU
}

func (m *mockSKURepo) Create(ctx context.Context, s *domainSKU.SKU) error {
	m.skus[s.SKU] = s
	return nil
}
func (m *mockSKURepo) FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	return nil, nil
}
func (m *mockSKURepo) FindBySKU(ctx context.Context, sku string) (*domainSKU.SKU, error) {
	if s, ok := m.skus[sku]; ok {
		return s, nil
	}
	return nil, nil
}
func (m *mockSKURepo) FindAll(ctx context.Context, q domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	return nil, 0, nil
}
func (m *mockSKURepo) Update(ctx context.Context, s *domainSKU.SKU) error {
	return nil
}
func (m *mockSKURepo) Delete(ctx context.Context, id uint) error {
	return nil
}
func (m *mockSKURepo) ExistsBySKU(ctx context.Context, sku string) (bool, error) {
	_, ok := m.skus[sku]
	return ok, nil
}
func (m *mockSKURepo) GetReceiptStatsBatch(ctx context.Context, skuIDs []uint) (map[uint]domainSKU.SKUBatchReceiptStat, error) {
	return make(map[uint]domainSKU.SKUBatchReceiptStat), nil
}
func (m *mockSKURepo) GetReceiptHistory(ctx context.Context, query domainSKU.SKUReceiptQuery) ([]domainSKU.SKUReceiptItem, int64, error) {
	return nil, 0, nil
}


func TestDeductionResolver_DirectSKU(t *testing.T) {
	formulaRepo := &mockFormulaRepo{formulas: make(map[string]*domainFormula.InventoryFormula)}
	skuRepo := &mockSKURepo{skus: make(map[string]*domainSKU.SKU)}
	resolver := usecaseStock.NewDeductionResolver(nil, formulaRepo, skuRepo)

	items := []usecaseStock.ItemToResolve{
		{SKU: "SKU-SINGLE", Quantity: 3, Price: 150},
	}

	res, err := resolver.ResolveDeductionItems(context.Background(), items, "ORDER", "SO-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res))
	}
	if res[0].SKUCode != "SKU-SINGLE" || res[0].Quantity != 3 {
		t.Errorf("expected SKU-SINGLE x 3, got %s x %d", res[0].SKUCode, res[0].Quantity)
	}
	if res[0].RefType != "DIRECT" {
		t.Errorf("expected RefType DIRECT, got %s", res[0].RefType)
	}
}

func TestDeductionResolver_FormulaExplosion(t *testing.T) {
	formulaRepo := &mockFormulaRepo{formulas: make(map[string]*domainFormula.InventoryFormula)}
	skuRepo := &mockSKURepo{skus: make(map[string]*domainSKU.SKU)}

	// Setup a formula: BUNDLE-SET = RAW-A x 2 + RAW-B x 1
	formulaRepo.formulas["BUNDLE-SET"] = &domainFormula.InventoryFormula{
		ID:       1,
		Code:     "BUNDLE-SET",
		Name:     "Combo Set",
		IsActive: true,
		Items: []domainFormula.InventoryFormulaItem{
			{ComponentSKU: "RAW-A", Qty: 2},
			{ComponentSKU: "RAW-B", Qty: 1},
		},
	}

	resolver := usecaseStock.NewDeductionResolver(nil, formulaRepo, skuRepo)

	items := []usecaseStock.ItemToResolve{
		{SKU: "bundle-set", Quantity: 5, Price: 500}, // Case-insensitive
	}

	res, err := resolver.ResolveDeductionItems(context.Background(), items, "TIKTOK", "TK-999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 2 {
		t.Fatalf("expected 2 components, got %d", len(res))
	}

	compMap := make(map[string]int)
	for _, itm := range res {
		compMap[itm.SKUCode] = itm.Quantity
		if itm.SourceFormulaCode != "BUNDLE-SET" {
			t.Errorf("expected SourceFormulaCode BUNDLE-SET, got %s", itm.SourceFormulaCode)
		}
		if itm.Channel != "TIKTOK" {
			t.Errorf("expected Channel TIKTOK, got %s", itm.Channel)
		}
	}

	if compMap["RAW-A"] != 10 { // 2 * 5
		t.Errorf("expected RAW-A x 10, got %d", compMap["RAW-A"])
	}
	if compMap["RAW-B"] != 5 { // 1 * 5
		t.Errorf("expected RAW-B x 5, got %d", compMap["RAW-B"])
	}
}

func TestDeductionResolver_AggregatesDuplicates(t *testing.T) {
	formulaRepo := &mockFormulaRepo{formulas: make(map[string]*domainFormula.InventoryFormula)}
	skuRepo := &mockSKURepo{skus: make(map[string]*domainSKU.SKU)}

	// Formula 1 uses RAW-A x 1
	formulaRepo.formulas["SET-1"] = &domainFormula.InventoryFormula{
		ID:       1,
		Code:     "SET-1",
		IsActive: true,
		Items: []domainFormula.InventoryFormulaItem{
			{ComponentSKU: "RAW-A", Qty: 1},
		},
	}
	// Formula 2 uses RAW-A x 3
	formulaRepo.formulas["SET-2"] = &domainFormula.InventoryFormula{
		ID:       2,
		Code:     "SET-2",
		IsActive: true,
		Items: []domainFormula.InventoryFormulaItem{
			{ComponentSKU: "RAW-A", Qty: 3},
		},
	}

	resolver := usecaseStock.NewDeductionResolver(nil, formulaRepo, skuRepo)

	items := []usecaseStock.ItemToResolve{
		{SKU: "SET-1", Quantity: 2}, // RAW-A x 2
		{SKU: "SET-2", Quantity: 1}, // RAW-A x 3
	}

	res, err := resolver.ResolveDeductionItems(context.Background(), items, "ORDER", "SO-MULTI")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalRawA := 0
	for _, itm := range res {
		if itm.SKUCode == "RAW-A" {
			totalRawA += itm.Quantity
		}
	}

	if totalRawA != 5 {
		t.Errorf("expected aggregated RAW-A x 5, got %d", totalRawA)
	}
}
