package sku

import "strings"

// Food flavors a single-bag SKU can carry. The CRM sales dashboard counts
// bags per flavor by expanding sold SKUs through their inventory formula into
// these components (plan CRM_BACKOFFICE_MARKETING.md §6). Empty = not a food
// bag (toys, accessories) or not set yet.
const (
	FlavorChicken = "chicken"
	FlavorTuna    = "tuna"
	FlavorRabbit  = "rabbit"
)

// NormalizeFlavor lower-cases and trims a flavor and reports whether it is
// allowed ("" is allowed and clears the flavor).
func NormalizeFlavor(v string) (string, bool) {
	f := strings.ToLower(strings.TrimSpace(v))
	switch f {
	case "", FlavorChicken, FlavorTuna, FlavorRabbit:
		return f, true
	}
	return f, false
}
