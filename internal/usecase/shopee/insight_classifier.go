package shopee

import (
	"regexp"
	"strconv"
	"strings"
)

// Product rules for the Shopee sales insights (spec "Chawy Sales Overview" §3).
// They read the listing name, the variation ("ชื่อตัวเลือก") and the SKU of an
// order line. All the keywords live here so they can be changed in one place.

const (
	CategoryFDMix   = "ฟรีซดรายโปรผสม"
	CategoryFDSolo  = "ฟรีซดรายชนิดเดี่ยว"
	CategoryCatToy  = "ของเล่นแมว"
	CategoryOther   = "อื่นๆ"
	MainFishingRod  = "ไม้ตกแมว"
	MainToyOrOther  = "ของเล่น/อื่นๆ"
	JourneyStopNode = "ยังไม่ซื้อต่อ"
)

// InsightCategories is the display order of the product categories.
var InsightCategories = []string{CategoryFDMix, CategoryFDSolo, CategoryCatToy, CategoryOther}

var (
	// Lines of the sister shop (แม่เพิ่ม pickles) are not Chawy sales.
	otherShopRe  = regexp.MustCompile(`(?i)แม่เพิ่ม|ขิงดอง|มะนาวดอง|กาน่าฉ่าย|pickled`)
	freezeDryRe  = regexp.MustCompile(`(?i)ฟรีซดราย|freeze[\s-]*dried`)
	promoBagsRe  = regexp.MustCompile(`โปร\s*(\d+)\s*ถุง`)
	bagsRe       = regexp.MustCompile(`(\d+)\s*ถุง`)
	weightRe     = regexp.MustCompile(`(?i)\d+(\.\d+)?\s*(g|กรัม|gram|ml|มล|kg|กก)`)
	numberRe     = regexp.MustCompile(`\d+`)
	skuPackRe    = regexp.MustCompile(`(?i)-(\d+)P(?:-|$)`)
	skuSuffixRe  = regexp.MustCompile(`-(\d{1,2})$`)
	fishingRodRe = regexp.MustCompile(`ไม้ตกแมว`)
	catToyRe     = regexp.MustCompile(`ไม้ตกแมว|หัวเปลี่ยน|ขนนก|หมอนแมว`)
)

// IsOtherShopProduct reports a line that belongs to the sister shop.
func IsOtherShopProduct(name string) bool { return otherShopRe.MatchString(name) }

// IsFreezeDried reports a freeze-dried snack line.
func IsFreezeDried(name string) bool { return freezeDryRe.MatchString(name) }

// SetSize is how many bags one unit of a freeze-dried line holds, read in the
// spec's order: listing "โปร N ถุง", variation "N ถุง", the sum of the numbers
// in the variation ("อกไก่ 2 ทูน่า 1" = 3), the SKU ("-03", "-10P-"), else 1.
// resolved is false only when it fell back to 1.
func SetSize(name, variation, sku string) (size int, resolved bool) {
	if n := firstInt(promoBagsRe, name); n > 0 {
		return n, true
	}
	if n := firstInt(bagsRe, variation); n > 0 {
		return n, true
	}
	if v := weightRe.ReplaceAllString(variation, " "); v != "" {
		sum := 0
		for _, m := range numberRe.FindAllString(v, -1) {
			n, _ := strconv.Atoi(m)
			sum += n
		}
		if sum > 0 {
			return sum, true
		}
	}
	sku = strings.TrimSpace(sku)
	if n := firstInt(skuPackRe, sku); n > 0 {
		return n, true
	}
	if n := firstInt(skuSuffixRe, sku); n > 0 {
		return n, true
	}
	return 1, false
}

// ProductCategory groups a line for the category chart and product table.
func ProductCategory(name string) string {
	switch {
	case IsFreezeDried(name) && promoBagsRe.MatchString(name):
		return CategoryFDMix
	case IsFreezeDried(name):
		return CategoryFDSolo
	case catToyRe.MatchString(name):
		return CategoryCatToy
	default:
		return CategoryOther
	}
}

// BagsLabel is the Sankey/set label of a freeze-dried set size.
func BagsLabel(size int) string {
	return "ฟรีซดราย " + strconv.Itoa(size) + " ถุง"
}

func firstInt(re *regexp.Regexp, s string) int {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
