package province

// Region names used by the CRM geographic report.
const (
	RegionBangkokMetro = "กรุงเทพฯ และปริมณฑล"
	RegionCentral      = "ภาคกลาง"
	RegionEast         = "ภาคตะวันออก"
	RegionNorth        = "ภาคเหนือ"
	RegionNortheast    = "ภาคอีสาน"
	RegionSouth        = "ภาคใต้"
	RegionWest         = "ภาคตะวันตก"
	RegionUnknown      = "ไม่ระบุภูมิภาค"
)

// regionOrder is the display order of regions.
var regionOrder = []string{
	RegionBangkokMetro, RegionCentral, RegionEast, RegionNorth,
	RegionNortheast, RegionSouth, RegionWest,
}

var regionProvinces = map[string][]string{
	RegionBangkokMetro: {"กรุงเทพมหานคร", "สมุทรปราการ", "นนทบุรี", "ปทุมธานี", "นครปฐม", "สมุทรสาคร"},
	RegionCentral:      {"พระนครศรีอยุธยา", "อ่างทอง", "ลพบุรี", "สิงห์บุรี", "ชัยนาท", "สระบุรี", "สุพรรณบุรี", "นครนายก"},
	RegionEast:         {"ชลบุรี", "ระยอง", "จันทบุรี", "ตราด", "ฉะเชิงเทรา", "ปราจีนบุรี", "สระแก้ว"},
	RegionNorth: {"เชียงใหม่", "ลำพูน", "ลำปาง", "อุตรดิตถ์", "แพร่", "น่าน", "พะเยา", "เชียงราย", "แม่ฮ่องสอน",
		"นครสวรรค์", "อุทัยธานี", "กำแพงเพชร", "ตาก", "สุโขทัย", "พิษณุโลก", "พิจิตร", "เพชรบูรณ์"},
	RegionNortheast: {"นครราชสีมา", "บุรีรัมย์", "สุรินทร์", "ศรีสะเกษ", "อุบลราชธานี", "ยโสธร", "ชัยภูมิ", "อำนาจเจริญ",
		"บึงกาฬ", "หนองบัวลำภู", "ขอนแก่น", "อุดรธานี", "เลย", "หนองคาย", "มหาสารคาม", "ร้อยเอ็ด", "กาฬสินธุ์",
		"สกลนคร", "นครพนม", "มุกดาหาร"},
	RegionSouth: {"ภูเก็ต", "สุราษฎร์ธานี", "กระบี่", "พังงา", "นครศรีธรรมราช", "ตรัง", "พัทลุง", "สงขลา", "สตูล",
		"ปัตตานี", "ยะลา", "นราธิวาส", "ชุมพร", "ระนอง"},
	RegionWest: {"ราชบุรี", "กาญจนบุรี", "สมุทรสงคราม", "เพชรบุรี", "ประจวบคีรีขันธ์"},
}

var provinceRegion map[string]string

func init() {
	provinceRegion = make(map[string]string, len(official77Provinces))
	for region, provs := range regionProvinces {
		for _, p := range provs {
			provinceRegion[p] = region
		}
	}
}

// RegionOf returns the region of an official Thai province name,
// or RegionUnknown when the province is not recognised.
func RegionOf(provinceName string) string {
	if r, ok := provinceRegion[provinceName]; ok {
		return r
	}
	return RegionUnknown
}

// Regions returns all known regions in display order.
func Regions() []string {
	res := make([]string, len(regionOrder))
	copy(res, regionOrder)
	return res
}

// TotalProvinces is the number of official Thai provinces.
func TotalProvinces() int { return len(official77Provinces) }
