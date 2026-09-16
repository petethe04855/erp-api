package stock

import (
	"context"
	"fmt"
	"strings"

	domainFormula "chawy-erp-api/internal/domain/formula"
	domainSKU "chawy-erp-api/internal/domain/sku"
	appErrors "chawy-erp-api/pkg/errors"
	"gorm.io/gorm"
)

type ItemToResolve struct {
	SKU      string
	Quantity int
	Price    float64
}

type ResolvedDeductionItem struct {
	SKUCode           string
	Quantity          int
	SourceFormulaCode string
	Channel           string
	RefType           string
	Note              string
}

type DeductionResolver interface {
	ResolveDeductionItems(
		ctx context.Context,
		items []ItemToResolve,
		channel string,
		referenceDocNo string,
	) ([]ResolvedDeductionItem, error)
}

type deductionResolver struct {
	db          *gorm.DB
	formulaRepo domainFormula.Repository
	skuRepo     domainSKU.Repository
}

func NewDeductionResolver(
	db *gorm.DB,
	formulaRepo domainFormula.Repository,
	skuRepo domainSKU.Repository,
) DeductionResolver {
	return &deductionResolver{
		db:          db,
		formulaRepo: formulaRepo,
		skuRepo:     skuRepo,
	}
}

func (r *deductionResolver) ResolveDeductionItems(
	ctx context.Context,
	items []ItemToResolve,
	channel string,
	referenceDocNo string,
) ([]ResolvedDeductionItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	deductionMap := make(map[string]*ResolvedDeductionItem)

	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, appErrors.NewAppError("INVALID_QUANTITY", fmt.Sprintf("Invalid quantity %d for SKU %s", item.Quantity, item.SKU), 400)
		}

		skuCode := strings.ToUpper(strings.TrimSpace(item.SKU))
		formula, err := r.formulaRepo.FindByCode(ctx, skuCode)
		if err != nil {
			return nil, err
		}

		if formula != nil && formula.IsActive && len(formula.Items) > 0 {
			// Explode formula components
			for _, fi := range formula.Items {
				compCode := strings.ToUpper(strings.TrimSpace(fi.ComponentSKU))
				neededQty := fi.Qty * item.Quantity
				key := fmt.Sprintf("%s|%s", compCode, formula.Code)
				if entry, exists := deductionMap[key]; exists {
					entry.Quantity += neededQty
				} else {
					deductionMap[key] = &ResolvedDeductionItem{
						SKUCode:           compCode,
						Quantity:          neededQty,
						SourceFormulaCode: formula.Code,
						Channel:           channel,
						RefType:           "FORMULA",
						Note:              fmt.Sprintf("Shipped for formula %s in %s %s", formula.Code, channel, referenceDocNo),
					}
				}
			}
		} else {
			// Direct SKU
			key := fmt.Sprintf("%s|", skuCode)
			if entry, exists := deductionMap[key]; exists {
				entry.Quantity += item.Quantity
			} else {
				deductionMap[key] = &ResolvedDeductionItem{
					SKUCode:           skuCode,
					Quantity:          item.Quantity,
					SourceFormulaCode: "",
					Channel:           channel,
					RefType:           "DIRECT",
					Note:              fmt.Sprintf("Shipped for %s %s", channel, referenceDocNo),
				}
			}

			// Explode Accessories if DB available
			if r.db != nil {
				var accessories []domainSKU.SKUAccessory
				if err := r.db.WithContext(ctx).Where("UPPER(sku) = ?", skuCode).Find(&accessories).Error; err == nil && len(accessories) > 0 {
					for _, acc := range accessories {
						accCode := strings.ToUpper(strings.TrimSpace(acc.AccessorySKU))
						accQty := acc.Quantity * item.Quantity
						accKey := fmt.Sprintf("%s|ACCESSORY|%s", accCode, skuCode)
						if entry, exists := deductionMap[accKey]; exists {
							entry.Quantity += accQty
						} else {
							deductionMap[accKey] = &ResolvedDeductionItem{
								SKUCode:           accCode,
								Quantity:          accQty,
								SourceFormulaCode: "",
								Channel:           channel,
								RefType:           "ACCESSORY",
								Note:              fmt.Sprintf("Shipped accessory %s for %s in %s %s", acc.AccessorySKU, skuCode, channel, referenceDocNo),
							}
						}
					}
				}
			}
		}
	}

	result := make([]ResolvedDeductionItem, 0, len(deductionMap))
	for _, v := range deductionMap {
		result = append(result, *v)
	}

	return result, nil
}
