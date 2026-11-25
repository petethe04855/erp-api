package shopee

import "testing"

func TestSetSize(t *testing.T) {
	cases := []struct {
		name, product, variation, sku string
		want                          int
		resolved                      bool
	}{
		{"promo listing wins", "[โปร 5 ถุง] อกไก่ ทูน่า ตับกระต่ายฟรีซดราย", "ไก่ 1 ทูน่า 2 ตับ 2", "FD-MX-C1T2R2-05", 5, true},
		{"spec conflict: listing โปร 3 ถุง beats variation อกไก่ 2 ทูน่า", "โปร 3 ถุง ฟรีซดราย", "อกไก่ 2 ทูน่า", "", 3, true},
		{"variation N ถุง", "อกไก่ฟรีซดราย", "3 ถุง", "", 3, true},
		{"sum of numbers in variation", "ฟรีซดราย ผสม", "อกไก่ 2 ทูน่า 1", "", 3, true},
		{"weights in variation are not bag counts", "อกไก่ฟรีซดราย", "100g", "FD-CK-100-01", 1, true},
		{"SKU pack size", "อกไก่ฟรีซดราย 100g", "", "FD-MX-10P-A", 10, true},
		{"SKU suffix", "ทูน่าฟรีซดราย", "", "FD-TN-03", 3, true},
		{"nothing found", "ทูน่าฟรีซดราย", "", "", 1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := SetSize(c.product, c.variation, c.sku)
			if got != c.want || ok != c.resolved {
				t.Fatalf("SetSize(%q, %q, %q) = %d, %v; want %d, %v", c.product, c.variation, c.sku, got, ok, c.want, c.resolved)
			}
		})
	}
}

func TestProductCategory(t *testing.T) {
	cases := map[string]string{
		"[โปร 5 ถุง] อกไก่ ทูน่า ฟรีซดราย": CategoryFDMix,
		"อกไก่ฟรีซดรายแท้ 100% ขนมแมว":     CategoryFDSolo,
		"Chicken Breast Freeze Dried":   CategoryFDSolo,
		"ไม้ตกแมว ขนนก":                 CategoryCatToy,
		"หมอนแมวแคทนิป100%ทำจากเชือกปอ": CategoryCatToy,
		"หัวเปลี่ยนขนนก":                CategoryCatToy,
		"กล่องพัสดุ":                    CategoryOther,
	}
	for name, want := range cases {
		if got := ProductCategory(name); got != want {
			t.Errorf("ProductCategory(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestIsOtherShopProduct(t *testing.T) {
	for _, n := range []string{
		"ขิงดอง(แม่เพิ่ม) 2 ปอนด์ น้ำหนัก 850 กรัม (Pickled Ginger)",
		"มะนาวดอง(ตราแม่เพิ่ม) 1 ปอนด์",
		"กาน่าฉ่าย สูตรน้ำมันรำข้าว(เจ) 190-500 กรัม",
	} {
		if !IsOtherShopProduct(n) {
			t.Errorf("%q should be the sister shop", n)
		}
	}
	for _, n := range []string{"อกไก่ฟรีซดราย", "หมอนแมวแคทนิป", "ไม้ตกแมว"} {
		if IsOtherShopProduct(n) {
			t.Errorf("%q is a Chawy product", n)
		}
	}
}
