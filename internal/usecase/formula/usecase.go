package formula

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainFormula "chawy-erp-api/internal/domain/formula"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	appErrors "chawy-erp-api/pkg/errors"
)

type ItemInput struct {
	ComponentSKU string `json:"componentSku"`
	Qty          int    `json:"qty"`
	Unit         string `json:"unit"`
}

type CreateFormulaInput struct {
	Code        string      `json:"code"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Image       string      `json:"image"`
	IsActive    *bool       `json:"isActive"`
	Items       []ItemInput `json:"items"`
}

type UpdateFormulaInput struct {
	Name        *string     `json:"name"`
	Description *string     `json:"description"`
	Image       *string     `json:"image"`
	IsActive    *bool       `json:"isActive"`
	Items       []ItemInput `json:"items"`
}

type FormulaItemResponse struct {
	ID           uint   `json:"id"`
	ComponentSKU string `json:"componentSku"`
	Qty          int    `json:"qty"`
	Unit         string `json:"unit"`
	AvailableQty int    `json:"availableQty"`
}

type FormulaResponse struct {
	ID            uint                  `json:"id"`
	Code          string                `json:"code"`
	Name          string                `json:"name"`
	Description   string                `json:"description"`
	Image         string                `json:"image"`
	IsActive      bool                  `json:"isActive"`
	AvailableSets int                   `json:"availableSets"`
	Items         []FormulaItemResponse `json:"items"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
}

type Usecase interface {
	Create(ctx context.Context, in CreateFormulaInput) (*FormulaResponse, error)
	Update(ctx context.Context, code string, in UpdateFormulaInput) (*FormulaResponse, error)
	Deactivate(ctx context.Context, code string) error
	ToggleStatus(ctx context.Context, code string, isActive bool) (*FormulaResponse, error)
	GetByCode(ctx context.Context, code string) (*FormulaResponse, error)
	List(ctx context.Context, query domainFormula.Query) ([]FormulaResponse, int64, error)
	ResolveFormulaItems(ctx context.Context, sku string, quantity int) ([]ItemInput, bool, error)
}

type formulaUsecase struct {
	formulaRepo domainFormula.Repository
	skuRepo     domainSKU.Repository
	stockRepo   domainStock.Repository
}

func NewFormulaUsecase(
	formulaRepo domainFormula.Repository,
	skuRepo domainSKU.Repository,
	stockRepo domainStock.Repository,
) Usecase {
	return &formulaUsecase{
		formulaRepo: formulaRepo,
		skuRepo:     skuRepo,
		stockRepo:   stockRepo,
	}
}

func (u *formulaUsecase) validateItems(ctx context.Context, formulaCode string, items []ItemInput) ([]domainFormula.InventoryFormulaItem, error) {
	if len(items) == 0 {
		return nil, appErrors.NewAppError("EMPTY_FORMULA_ITEMS", "สูตรต้องมีส่วนประกอบอย่างน้อย 1 รายการ", 400)
	}

	normCode := strings.ToUpper(strings.TrimSpace(formulaCode))
	seenComponents := make(map[string]bool)
	formulaItems := make([]domainFormula.InventoryFormulaItem, 0, len(items))

	for _, item := range items {
		compSKU := strings.ToUpper(strings.TrimSpace(item.ComponentSKU))
		if compSKU == "" {
			return nil, appErrors.NewAppError("INVALID_COMPONENT_SKU", "รหัสสินค้าส่วนประกอบต้องไม่ว่างเปล่า", 400)
		}
		if compSKU == normCode {
			return nil, appErrors.NewAppError("SELF_REFERENCE_NOT_ALLOWED", fmt.Sprintf("สูตรห้ามอ้างถึงตัวเอง (%s)", compSKU), 400)
		}
		if item.Qty <= 0 {
			return nil, appErrors.NewAppError("INVALID_COMPONENT_QTY", fmt.Sprintf("จำนวนของส่วนประกอบ %s ต้องมากกว่า 0", compSKU), 400)
		}
		if seenComponents[compSKU] {
			return nil, appErrors.NewAppError("DUPLICATE_COMPONENT", fmt.Sprintf("ส่วนประกอบ %s ซ้ำในสูตรเดียวกัน", compSKU), 400)
		}
		seenComponents[compSKU] = true

		// Check component SKU exists in SKU Master
		skuEntity, err := u.skuRepo.FindBySKU(ctx, compSKU)
		if err != nil {
			return nil, err
		}
		if skuEntity == nil {
			return nil, appErrors.NewAppError("COMPONENT_SKU_NOT_FOUND", fmt.Sprintf("ไม่พบสินค้าส่วนประกอบ %s ใน SKU Master", compSKU), 404)
		}

		// Single tier rule: Component SKU must NOT be another formula (ห้ามอ้างถึงสูตรอื่น)
		otherFormula, err := u.formulaRepo.FindByCode(ctx, compSKU)
		if err != nil {
			return nil, err
		}
		if otherFormula != nil && otherFormula.IsActive {
			return nil, appErrors.NewAppError("NESTED_FORMULA_NOT_ALLOWED", fmt.Sprintf("สูตรรองรับหนึ่งชั้นเท่านั้น: %s เป็นสูตรตัดสต็อกอยู่แล้ว ไม่สามารถนำมาเป็นส่วนประกอบได้", compSKU), 400)
		}

		formulaItems = append(formulaItems, domainFormula.InventoryFormulaItem{
			FormulaCode:  normCode,
			ComponentSKU: compSKU,
			Qty:          item.Qty,
			Unit:         strings.TrimSpace(item.Unit),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		})
	}

	return formulaItems, nil
}

func (u *formulaUsecase) calculateAvailableSets(ctx context.Context, items []domainFormula.InventoryFormulaItem) (int, []FormulaItemResponse, error) {
	if len(items) == 0 {
		return 0, nil, nil
	}

	itemResponses := make([]FormulaItemResponse, len(items))
	minSets := -1

	for i, itm := range items {
		skuEntity, err := u.skuRepo.FindBySKU(ctx, itm.ComponentSKU)
		availQty := 0
		if err == nil && skuEntity != nil {
			stk, err := u.stockRepo.GetBySKUID(ctx, skuEntity.ID, 1)
			if err == nil && stk != nil {
				availQty = stk.AvailableQty
			}
		}

		sets := availQty / itm.Qty
		if minSets == -1 || sets < minSets {
			minSets = sets
		}

		itemResponses[i] = FormulaItemResponse{
			ID:           itm.ID,
			ComponentSKU: itm.ComponentSKU,
			Qty:          itm.Qty,
			Unit:         itm.Unit,
			AvailableQty: availQty,
		}
	}

	if minSets < 0 {
		minSets = 0
	}

	return minSets, itemResponses, nil
}

func (u *formulaUsecase) Create(ctx context.Context, in CreateFormulaInput) (*FormulaResponse, error) {
	normCode := strings.ToUpper(strings.TrimSpace(in.Code))
	if normCode == "" {
		return nil, appErrors.NewAppError("EMPTY_FORMULA_CODE", "กรุณาระบุรหัสสูตรตัดสต็อก (Code)", 400)
	}

	exists, err := u.formulaRepo.ExistsByCode(ctx, normCode)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, appErrors.NewAppError("FORMULA_ALREADY_EXISTS", fmt.Sprintf("รหัสสูตร %s มีอยู่ในระบบแล้ว", normCode), 409)
	}

	formulaItems, err := u.validateItems(ctx, normCode, in.Items)
	if err != nil {
		return nil, err
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	formula := &domainFormula.InventoryFormula{
		Code:        normCode,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Image:       strings.TrimSpace(in.Image),
		IsActive:    isActive,
		Items:       formulaItems,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := u.formulaRepo.Create(ctx, formula); err != nil {
		return nil, err
	}

	availSets, itemRes, _ := u.calculateAvailableSets(ctx, formulaItems)

	return &FormulaResponse{
		ID:            formula.ID,
		Code:          formula.Code,
		Name:          formula.Name,
		Description:   formula.Description,
		Image:         formula.Image,
		IsActive:      formula.IsActive,
		AvailableSets: availSets,
		Items:         itemRes,
		CreatedAt:     formula.CreatedAt,
		UpdatedAt:     formula.UpdatedAt,
	}, nil
}

func (u *formulaUsecase) Update(ctx context.Context, code string, in UpdateFormulaInput) (*FormulaResponse, error) {
	normCode := strings.ToUpper(strings.TrimSpace(code))
	existing, err := u.formulaRepo.FindByCode(ctx, normCode)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, appErrors.NewAppError("FORMULA_NOT_FOUND", fmt.Sprintf("ไม่พบสูตรตัดสต็อก %s", normCode), 404)
	}

	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		existing.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		existing.Description = strings.TrimSpace(*in.Description)
	}
	if in.Image != nil {
		existing.Image = strings.TrimSpace(*in.Image)
	}
	if in.IsActive != nil {
		existing.IsActive = *in.IsActive
	}
	existing.UpdatedAt = time.Now()

	if len(in.Items) > 0 {
		formulaItems, err := u.validateItems(ctx, normCode, in.Items)
		if err != nil {
			return nil, err
		}
		existing.Items = formulaItems
	}

	if err := u.formulaRepo.Update(ctx, existing); err != nil {
		return nil, err
	}

	availSets, itemRes, _ := u.calculateAvailableSets(ctx, existing.Items)

	return &FormulaResponse{
		ID:            existing.ID,
		Code:          existing.Code,
		Name:          existing.Name,
		Description:   existing.Description,
		Image:         existing.Image,
		IsActive:      existing.IsActive,
		AvailableSets: availSets,
		Items:         itemRes,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     existing.UpdatedAt,
	}, nil
}

func (u *formulaUsecase) Deactivate(ctx context.Context, code string) error {
	normCode := strings.ToUpper(strings.TrimSpace(code))
	existing, err := u.formulaRepo.FindByCode(ctx, normCode)
	if err != nil {
		return err
	}
	if existing == nil {
		return appErrors.NewAppError("FORMULA_NOT_FOUND", fmt.Sprintf("ไม่พบสูตรตัดสต็อก %s", normCode), 404)
	}
	return u.formulaRepo.Deactivate(ctx, normCode)
}

func (u *formulaUsecase) ToggleStatus(ctx context.Context, code string, isActive bool) (*FormulaResponse, error) {
	normCode := strings.ToUpper(strings.TrimSpace(code))
	existing, err := u.formulaRepo.FindByCode(ctx, normCode)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, appErrors.NewAppError("FORMULA_NOT_FOUND", fmt.Sprintf("ไม่พบสูตรตัดสต็อก %s", normCode), 404)
	}

	if err := u.formulaRepo.ToggleStatus(ctx, normCode, isActive); err != nil {
		return nil, err
	}

	existing.IsActive = isActive
	availSets, itemRes, _ := u.calculateAvailableSets(ctx, existing.Items)

	return &FormulaResponse{
		ID:            existing.ID,
		Code:          existing.Code,
		Name:          existing.Name,
		Description:   existing.Description,
		Image:         existing.Image,
		IsActive:      existing.IsActive,
		AvailableSets: availSets,
		Items:         itemRes,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     existing.UpdatedAt,
	}, nil
}

func (u *formulaUsecase) GetByCode(ctx context.Context, code string) (*FormulaResponse, error) {
	normCode := strings.ToUpper(strings.TrimSpace(code))
	existing, err := u.formulaRepo.FindByCode(ctx, normCode)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, appErrors.NewAppError("FORMULA_NOT_FOUND", fmt.Sprintf("ไม่พบสูตรตัดสต็อก %s", normCode), 404)
	}

	availSets, itemRes, _ := u.calculateAvailableSets(ctx, existing.Items)

	return &FormulaResponse{
		ID:            existing.ID,
		Code:          existing.Code,
		Name:          existing.Name,
		Description:   existing.Description,
		Image:         existing.Image,
		IsActive:      existing.IsActive,
		AvailableSets: availSets,
		Items:         itemRes,
		CreatedAt:     existing.CreatedAt,
		UpdatedAt:     existing.UpdatedAt,
	}, nil
}

func (u *formulaUsecase) List(ctx context.Context, query domainFormula.Query) ([]FormulaResponse, int64, error) {
	formulas, total, err := u.formulaRepo.FindAll(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]FormulaResponse, len(formulas))
	for i, f := range formulas {
		availSets, itemRes, _ := u.calculateAvailableSets(ctx, f.Items)
		responses[i] = FormulaResponse{
			ID:            f.ID,
			Code:          f.Code,
			Name:          f.Name,
			Description:   f.Description,
			Image:         f.Image,
			IsActive:      f.IsActive,
			AvailableSets: availSets,
			Items:         itemRes,
			CreatedAt:     f.CreatedAt,
			UpdatedAt:     f.UpdatedAt,
		}
	}

	return responses, total, nil
}

// ResolveFormulaItems returns exploded components x quantity if an active formula exists for sku.
// If no formula is found or formula is inactive, it returns found=false.
func (u *formulaUsecase) ResolveFormulaItems(ctx context.Context, sku string, quantity int) ([]ItemInput, bool, error) {
	normSKU := strings.ToUpper(strings.TrimSpace(sku))
	f, err := u.formulaRepo.FindByCode(ctx, normSKU)
	if err != nil {
		return nil, false, err
	}
	if f == nil || !f.IsActive || len(f.Items) == 0 {
		return nil, false, nil
	}

	out := make([]ItemInput, len(f.Items))
	for i, itm := range f.Items {
		out[i] = ItemInput{
			ComponentSKU: itm.ComponentSKU,
			Qty:          itm.Qty * quantity,
			Unit:         itm.Unit,
		}
	}

	return out, true, nil
}
