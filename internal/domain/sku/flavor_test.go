package sku

import "testing"

func TestNormalizeFlavor(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{
		"":          {"", true},
		" Chicken ": {FlavorChicken, true},
		"TUNA":      {FlavorTuna, true},
		"rabbit":    {FlavorRabbit, true},
		"beef":      {"beef", false},
	}
	for in, c := range cases {
		got, ok := NormalizeFlavor(in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeFlavor(%q) = %q, %v; want %q, %v", in, got, ok, c.want, c.ok)
		}
	}
}
