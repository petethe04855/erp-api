package province_test

import (
	"testing"

	"chawy-erp-api/pkg/province"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeProvince(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		postalCode string
		expected   string
	}{
		{"Exact Thai match", "กรุงเทพมหานคร", "", "กรุงเทพมหานคร"},
		{"Abbreviation กทม.", "กทม.", "", "กรุงเทพมหานคร"},
		{"Abbreviation กทม", "กทม", "", "กรุงเทพมหานคร"},
		{"Colloquial กรุงเทพ", "กรุงเทพ", "", "กรุงเทพมหานคร"},
		{"English Bangkok", "Bangkok", "", "กรุงเทพมหานคร"},
		{"English BKK", "BKK", "", "กรุงเทพมหานคร"},
		{"English Chiang Mai", "Chiang Mai", "", "เชียงใหม่"},
		{"English Chiangmai", "Chiangmai", "", "เชียงใหม่"},
		{"Thai Nonthaburi", "นนทบุรี", "", "นนทบุรี"},
		{"English Nonthaburi", "Nonthaburi", "", "นนทบุรี"},
		{"Thai Chonburi", "ชลบุรี", "", "ชลบุรี"},
		{"English Chonburi", "Chonburi", "", "ชลบุรี"},
		{"Thai Phuket", "ภูเก็ต", "", "ภูเก็ต"},
		{"English Phuket", "Phuket", "", "ภูเก็ต"},
		{"Fallback Postal Code 10110", "", "10110", "กรุงเทพมหานคร"},
		{"Fallback Postal Code 20000", "", "20000", "ชลบุรี"},
		{"Fallback Postal Code 50000", "", "50000", "เชียงใหม่"},
		{"Fallback Postal Code 83000", "", "83000", "ภูเก็ต"},
		{"Whitespace trimming", "  กรุงเทพมหานคร  ", "", "กรุงเทพมหานคร"},
		{"Unknown Empty", "", "", "ไม่ทราบจังหวัด"},
		{"Unknown Invalid", "Atlantis", "99999", "ไม่ทราบจังหวัด"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := province.NormalizeProvince(tt.raw, tt.postalCode)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestAllProvinces(t *testing.T) {
	all := province.AllProvinces()
	assert.Equal(t, 77, len(all), "Thailand has 77 provinces including Bangkok")
	assert.Contains(t, all, "กรุงเทพมหานคร")
	assert.Contains(t, all, "เชียงใหม่")
	assert.Contains(t, all, "ภูเก็ต")
}
