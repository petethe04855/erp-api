package salesanalytics

import (
	"context"
	"strings"

	domainFormula "chawy-erp-api/internal/domain/formula"
	"chawy-erp-api/internal/domain/salesanalytics"
	domainSKU "chawy-erp-api/internal/domain/sku"
)

// FormulaFinder reads inventory formulas (the formula repository).
type FormulaFinder interface {
	FindByCode(ctx context.Context, code string) (*domainFormula.InventoryFormula, error)
}

// SKUFinder reads SKUs by code (the SKU repository).
type SKUFinder interface {
	FindBySKU(ctx context.Context, skuCode string) (*domainSKU.SKU, error)
}

// Option configures the usecase.
type Option func(*usecase)

// WithFlavorSources enables bag counting per flavor. Without it the bag
// fields stay zero, as before.
func WithFlavorSources(formulas FormulaFinder, skus SKUFinder) Option {
	return func(u *usecase) {
		u.formulas = formulas
		u.skus = skus
	}
}

// MixedFlavor marks a line whose bags span more than one flavor.
const MixedFlavor = "mixed"

// bagCounter expands sold SKUs into single bags the way stock deduction does:
// a SKU with an active inventory formula becomes its components × qty, any
// other SKU counts as itself. Each component's flavor comes from the SKU
// master (skus.flavor). Lookups are cached for one request.
type bagCounter struct {
	formulas FormulaFinder
	skus     SKUFinder
	formula  map[string]*domainFormula.InventoryFormula
	flavor   map[string]string
}

func newBagCounter(formulas FormulaFinder, skus SKUFinder) *bagCounter {
	return &bagCounter{formulas: formulas, skus: skus,
		formula: map[string]*domainFormula.InventoryFormula{}, flavor: map[string]string{}}
}

func normCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

func (b *bagCounter) flavorOf(ctx context.Context, code string) (string, error) {
	if f, ok := b.flavor[code]; ok {
		return f, nil
	}
	s, err := b.skus.FindBySKU(ctx, code)
	if err != nil {
		return "", err
	}
	f := ""
	if s != nil {
		f, _ = domainSKU.NormalizeFlavor(s.Flavor)
	}
	b.flavor[code] = f
	return f, nil
}

func (b *bagCounter) formulaOf(ctx context.Context, code string) (*domainFormula.InventoryFormula, error) {
	if f, ok := b.formula[code]; ok {
		return f, nil
	}
	f, err := b.formulas.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if f != nil && (!f.IsActive || len(f.Items) == 0) {
		f = nil
	}
	b.formula[code] = f
	return f, nil
}

// lineBags returns the bags per flavor of one order line.
func (b *bagCounter) lineBags(ctx context.Context, l salesanalytics.Line) (map[string]int, error) {
	out := map[string]int{}
	if l.Qty <= 0 {
		return out, nil
	}
	code := normCode(l.SKU)
	if code == "" {
		return out, nil
	}
	f, err := b.formulaOf(ctx, code)
	if err != nil {
		return nil, err
	}
	if f == nil {
		flavor, err := b.flavorOf(ctx, code)
		if err != nil {
			return nil, err
		}
		if flavor != "" {
			out[flavor] += l.Qty
		}
		return out, nil
	}
	for _, item := range f.Items {
		flavor, err := b.flavorOf(ctx, normCode(item.ComponentSKU))
		if err != nil {
			return nil, err
		}
		if flavor != "" {
			out[flavor] += item.Qty * l.Qty
		}
	}
	return out, nil
}

// apply fills the bag fields of every order and the flavor of every line.
func (b *bagCounter) apply(ctx context.Context, orders []salesanalytics.Order) error {
	for i := range orders {
		o := &orders[i]
		o.ChickenBags, o.TunaBags, o.RabbitBags, o.TotalBags = 0, 0, 0, 0
		for j := range o.Lines {
			bags, err := b.lineBags(ctx, o.Lines[j])
			if err != nil {
				return err
			}
			o.Lines[j].Flavor = ""
			for flavor, n := range bags {
				switch flavor {
				case domainSKU.FlavorChicken:
					o.ChickenBags += n
				case domainSKU.FlavorTuna:
					o.TunaBags += n
				case domainSKU.FlavorRabbit:
					o.RabbitBags += n
				}
				o.TotalBags += n
				if o.Lines[j].Flavor == "" {
					o.Lines[j].Flavor = flavor
				} else if o.Lines[j].Flavor != flavor {
					o.Lines[j].Flavor = MixedFlavor
				}
			}
		}
	}
	return nil
}
