package booth

import (
	"fmt"
	"math"
	"sort"
	"strings"

	domainBooth "chawy-erp-api/internal/domain/booth"
	appErrors "chawy-erp-api/pkg/errors"
)

// Bill is a priced cart. The API prices every bill itself; the booth screen
// only shows a preview computed the same way.
type Bill struct {
	Bags       int                     `json:"bags"`
	Lines      []domainBooth.SaleLine  `json:"lines"`
	TreatParts []domainBooth.TreatPart `json:"treatParts"`
	TreatLabel string                  `json:"treatLabel"`
	TreatTotal float64                 `json:"treatTotal"`
	ItemsTotal float64                 `json:"itemsTotal"`
	Subtotal   float64                 `json:"subtotal"`
	Discount   float64                 `json:"discount"`
	Total      float64                 `json:"total"`
}

// BestSetBreakdown is the cheapest way to price `bags` bags with the set
// prices (unbounded knapsack); on a tie it uses fewer sets. A set of 1 must
// exist so every count can be priced.
func BestSetBreakdown(bags int, sets []domainBooth.SetPrice) ([]domainBooth.TreatPart, float64, error) {
	if bags < 0 {
		return nil, 0, appErrors.NewAppError("VALIDATION_ERROR", "invalid number of bags", 400)
	}
	if bags == 0 {
		return []domainBooth.TreatPart{}, 0, nil
	}
	hasOne := false
	valid := make([]domainBooth.SetPrice, 0, len(sets))
	for _, s := range sets {
		if s.Size > 0 && s.Price >= 0 {
			valid = append(valid, s)
			if s.Size == 1 {
				hasOne = true
			}
		}
	}
	if !hasOne {
		return nil, 0, appErrors.NewAppError("BOOTH_CONFIG_INVALID", "set prices need a set of 1 bag", 400)
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].Size < valid[j].Size })

	cost := make([]float64, bags+1)
	pieces := make([]int, bags+1)
	pick := make([]int, bags+1)
	for i := 1; i <= bags; i++ {
		cost[i] = math.Inf(1)
		pieces[i] = math.MaxInt32
		pick[i] = -1
		for k, s := range valid {
			if s.Size > i {
				continue
			}
			c := cost[i-s.Size] + s.Price
			p := pieces[i-s.Size] + 1
			if c < cost[i]-1e-9 || (math.Abs(c-cost[i]) < 1e-9 && p < pieces[i]) {
				cost[i], pieces[i], pick[i] = c, p, k
			}
		}
	}

	counts := map[int]*domainBooth.TreatPart{}
	for i := bags; i > 0; {
		s := valid[pick[i]]
		if counts[s.Size] == nil {
			counts[s.Size] = &domainBooth.TreatPart{Size: s.Size, UnitPrice: s.Price}
		}
		counts[s.Size].Count++
		i -= s.Size
	}
	parts := make([]domainBooth.TreatPart, 0, len(counts))
	for _, p := range counts {
		parts = append(parts, *p)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Size > parts[j].Size })
	return parts, round2(cost[bags]), nil
}

// SetLabel reads like "เซต 5 + เซต 1 ×2".
func SetLabel(parts []domainBooth.TreatPart) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.Count > 1 {
			out = append(out, fmt.Sprintf("เซต %d ×%d", p.Size, p.Count))
		} else {
			out = append(out, fmt.Sprintf("เซต %d", p.Size))
		}
	}
	return strings.Join(out, " + ")
}

// PriceCart prices quantities keyed by SKU id against the config. SKUs that
// are not booth buttons are refused. Lines come back in button order.
func PriceCart(cfg domainBooth.Config, qty map[uint]int) (*Bill, error) {
	known := map[uint]bool{}
	bill := &Bill{Lines: []domainBooth.SaleLine{}}
	for _, f := range cfg.Flavors {
		known[f.SKUID] = true
		if q := qty[f.SKUID]; q > 0 {
			bill.Lines = append(bill.Lines, domainBooth.SaleLine{SKUID: f.SKUID, Name: f.Name, Kind: domainBooth.KindFlavor, Qty: q})
			bill.Bags += q
		}
	}
	for _, it := range cfg.Items {
		known[it.SKUID] = true
		if q := qty[it.SKUID]; q > 0 {
			amount := round2(it.Price * float64(q))
			bill.Lines = append(bill.Lines, domainBooth.SaleLine{SKUID: it.SKUID, Name: it.Name, Kind: domainBooth.KindItem, Qty: q, UnitPrice: it.Price, Amount: amount})
			bill.ItemsTotal += amount
		}
	}
	for id, q := range qty {
		if q > 0 && !known[id] {
			return nil, appErrors.NewAppError("VALIDATION_ERROR", fmt.Sprintf("SKU %d is not sold at the booth", id), 400)
		}
	}
	parts, treat, err := BestSetBreakdown(bill.Bags, cfg.Sets)
	if err != nil {
		return nil, err
	}
	bill.TreatParts, bill.TreatTotal, bill.TreatLabel = parts, treat, SetLabel(parts)
	// A flavor line carries its share of the set money, split by bags, so the
	// lines add up to the subtotal.
	if bill.Bags > 0 {
		for i := range bill.Lines {
			if bill.Lines[i].Kind == domainBooth.KindFlavor {
				bill.Lines[i].Amount = round2(treat * float64(bill.Lines[i].Qty) / float64(bill.Bags))
			}
		}
	}
	bill.ItemsTotal = round2(bill.ItemsTotal)
	bill.Subtotal = round2(bill.TreatTotal + bill.ItemsTotal)

	d := cfg.Discount
	if d.Enabled && bill.Subtotal > 0 && bill.Subtotal >= d.Min {
		raw := d.Value
		if d.Type == domainBooth.DiscountPct {
			raw = math.Round(bill.Subtotal * d.Value / 100)
		}
		bill.Discount = round2(math.Min(math.Max(0, raw), bill.Subtotal))
	}
	bill.Total = round2(bill.Subtotal - bill.Discount)
	return bill, nil
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
