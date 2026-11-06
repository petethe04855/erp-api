package province

import (
	"strings"
)

const UnknownProvince = "ไม่ทราบจังหวัด"

// official77Provinces contains all 77 official Thai provinces
var official77Provinces = []string{
	"กรุงเทพมหานคร", "กระบี่", "กาญจนบุรี", "กาฬสินธุ์", "กำแพงเพชร",
	"ขอนแก่น", "จันทบุรี", "ฉะเชิงเทรา", "ชลบุรี", "ชัยนาท",
	"ชัยภูมิ", "ชุมพร", "เชียงราย", "เชียงใหม่", "ตรัง",
	"ตราด", "ตาก", "นครนายก", "นครปฐม", "นครพนม",
	"นครราชสีมา", "นครศรีธรรมราช", "นครสวรรค์", "นนทบุรี", "นราธิวาส",
	"น่าน", "บึงกาฬ", "บุรีรัมย์", "ปทุมธานี", "ประจวบคีรีขันธ์",
	"ปราจีนบุรี", "ปัตตานี", "พระนครศรีอยุธยา", "พังงา", "พัทลุง",
	"พิจิตร", "พิษณุโลก", "เพชรบุรี", "เพชรบูรณ์", "แพร่",
	"พะเยา", "ภูเก็ต", "มหาสารคาม", "มุกดาหาร", "แม่ฮ่องสอน",
	"ยะลา", "ยโสธร", "ร้อยเอ็ด", "ระนอง", "ระยอง",
	"ราชบุรี", "ลพบุรี", "ลำปาง", "ลำพูน", "เลย",
	"ศรีสะเกษ", "สกลนคร", "สงขลา", "สตูล", "สมุทรปราการ",
	"สมุทรสงคราม", "สมุทรสาคร", "สระแก้ว", "สระบุรี", "สิงห์บุรี",
	"สุโขทัย", "สุพรรณบุรี", "สุราษฎร์ธานี", "สุรินทร์", "หนองคาย",
	"หนองบัวลำภู", "อ่างทอง", "อำนาจเจริญ", "อุดรธานี", "อุตรดิตถ์",
	"อุทัยธานี", "อุบลราชธานี",
}

// aliasMap maps aliases, English names, abbreviations to official Thai province names
var aliasMap = map[string]string{
	// Bangkok variations
	"bangkok":       "กรุงเทพมหานคร",
	"bkk":           "กรุงเทพมหานคร",
	"กทม":           "กรุงเทพมหานคร",
	"กทม.":          "กรุงเทพมหานคร",
	"กรุงเทพ":       "กรุงเทพมหานคร",
	"กรุงเทพฯ":      "กรุงเทพมหานคร",
	"กรุงเทพมหานคร": "กรุงเทพมหานคร",

	// Central
	"nonthaburi":           "นนทบุรี",
	"pathum thani":         "ปทุมธานี",
	"pathumthani":          "ปทุมธานี",
	"samut prakan":         "สมุทรปราการ",
	"samutprakan":          "สมุทรปราการ",
	"samut sakhon":         "สมุทรสาคร",
	"samutsakhon":          "สมุทรสาคร",
	"samut songkhram":      "สมุทรสงคราม",
	"samutsongkhram":       "สมุทรสงคราม",
	"phra nakhon si ayutthaya": "พระนครศรีอยุธยา",
	"ayutthaya":            "พระนครศรีอยุธยา",
	"อยุธยา":               "พระนครศรีอยุธยา",
	"ang thong":            "อ่างทอง",
	"angthong":             "อ่างทอง",
	"lopburi":              "ลพบุรี",
	"sing buri":            "สิงห์บุรี",
	"singburi":             "สิงห์บุรี",
	"chai nat":             "ชัยนาท",
	"chainat":              "ชัยนาท",
	"saraburi":             "สระบุรี",
	"nakhon nayok":         "นครนายก",
	"nakhonnayok":          "นครนายก",

	// East
	"chonburi":             "ชลบุรี",
	"chon buri":            "ชลบุรี",
	"rayong":               "ระยอง",
	"chanthaburi":          "จันทบุรี",
	"trat":                 "ตราด",
	"chachoengsao":         "ฉะเชิงเทรา",
	"prachinburi":          "ปราจีนบุรี",
	"sa kaeo":              "สระแก้ว",
	"sakaeo":               "สระแก้ว",

	// North
	"chiang mai":           "เชียงใหม่",
	"chiangmai":            "เชียงใหม่",
	"chiang rai":           "เชียงราย",
	"chiangrai":            "เชียงราย",
	"lampang":              "ลำปาง",
	"lamphun":              "ลำพูน",
	"mae hong son":         "แม่ฮ่องสอน",
	"maehongson":           "แม่ฮ่องสอน",
	"nan":                  "น่าน",
	"phayao":               "พะเยา",
	"phrae":                "แพร่",
	"uttaradit":            "อุตรดิตถ์",
	"tak":                  "ตาก",
	"sukhothai":            "สุโขทัย",
	"phitsanulok":          "พิษณุโลก",
	"kamphaeng phet":       "กำแพงเพชร",
	"kamphaengphet":        "กำแพงเพชร",
	"phichit":              "พิจิตร",
	"phetchabun":           "เพชรบูรณ์",
	"nakhon sawan":         "นครสวรรค์",
	"nakhonsawan":          "นครสวรรค์",
	"uthai thani":          "อุทัยธานี",
	"uthaithani":           "อุทัยธานี",

	// Northeast (Isan)
	"nakhon ratchasima":    "นครราชสีมา",
	"nakhonratchasima":     "นครราชสีมา",
	"korat":                "นครราชสีมา",
	"โคราช":                "นครราชสีมา",
	"buriram":              "บุรีรัมย์",
	"buri ram":             "บุรีรัมย์",
	"surin":                "สุรินทร์",
	"sisaket":              "ศรีสะเกษ",
	"si sa ket":            "ศรีสะเกษ",
	"ubon ratchathani":     "อุบลราชธานี",
	"ubonratchathani":      "อุบลราชธานี",
	"yasothon":             "ยโสธร",
	"chaiyaphum":           "ชัยภูมิ",
	"amnat charoen":        "อำนาจเจริญ",
	"amnatcharoen":         "อำนาจเจริญ",
	"bueng kan":            "บึงกาฬ",
	"buengkan":             "บึงกาฬ",
	"nong bua lamphu":      "หนองบัวลำภู",
	"nongbualamphu":        "หนองบัวลำภู",
	"khon kaen":            "ขอนแก่น",
	"khonkaen":             "ขอนแก่น",
	"udon thani":           "อุดรธานี",
	"udonthani":            "อุดรธานี",
	"loei":                 "เลย",
	"nong khai":            "หนองคาย",
	"nongkhai":             "หนองคาย",
	"maha sarakham":        "มหาสารคาม",
	"mahasarakham":         "มหาสารคาม",
	"roi et":               "ร้อยเอ็ด",
	"roiet":                "ร้อยเอ็ด",
	"kalasin":              "กาฬสินธุ์",
	"sakon nakhon":         "สกลนคร",
	"sakonnakhon":          "สกลนคร",
	"nakhon phanom":        "นครพนม",
	"nakhonphanom":         "นครพนม",
	"mukdahan":             "มุกดาหาร",

	// West
	"kanchanaburi":         "กาญจนบุรี",
	"ratchaburi":           "ราชบุรี",
	"suphan buri":          "สุพรรณบุรี",
	"suphanburi":           "สุพรรณบุรี",
	"nakhon pathom":        "นครปฐม",
	"nakhonpathom":         "นครปฐม",
	"phetchaburi":          "เพชรบุรี",
	"prachuap khiri khan":  "ประจวบคีรีขันธ์",
	"prachuap":             "ประจวบคีรีขันธ์",

	// South
	"chumphon":             "ชุมพร",
	"ranong":               "ระนอง",
	"surat thani":          "สุราษฎร์ธานี",
	"suratthani":           "สุราษฎร์ธานี",
	"phang nga":            "พังงา",
	"phangnga":             "พังงา",
	"phuket":               "ภูเก็ต",
	"krabi":                "กระบี่",
	"nakhon si thammarat":  "นครศรีธรรมราช",
	"nakhonsithammarat":   "นครศรีธรรมราช",
	"trang":                "ตรัง",
	"phatthalung":          "พัทลุง",
	"satun":                "สตูล",
	"songkhla":             "สงขลา",
	"pattani":              "ปัตตานี",
	"yala":                 "ยะลา",
	"narathiwat":           "นราธิวาส",
}

// postalCode2DigitPrefixMap maps 2-digit postal code prefixes to official Thai provinces
var postalCode2DigitPrefixMap = map[string]string{
	"10": "กรุงเทพมหานคร",
	"11": "นนทบุรี",
	"12": "ปทุมธานี",
	"13": "พระนครศรีอยุธยา",
	"14": "อ่างทอง",
	"15": "ลพบุรี",
	"16": "สิงห์บุรี",
	"17": "ชัยนาท",
	"18": "สระบุรี",
	"20": "ชลบุรี",
	"21": "ระยอง",
	"22": "จันทบุรี",
	"23": "ตราด",
	"24": "ฉะเชิงเทรา",
	"25": "ปราจีนบุรี",
	"26": "นครนายก",
	"27": "สระแก้ว",
	"30": "นครราชสีมา",
	"31": "บุรีรัมย์",
	"32": "สุรินทร์",
	"33": "ศรีสะเกษ",
	"34": "อุบลราชธานี",
	"35": "ยโสธร",
	"36": "ชัยภูมิ",
	"37": "อำนาจเจริญ",
	"38": "บึงกาฬ",
	"39": "หนองบัวลำภู",
	"40": "ขอนแก่น",
	"41": "อุดรธานี",
	"42": "เลย",
	"43": "หนองคาย",
	"44": "มหาสารคาม",
	"45": "ร้อยเอ็ด",
	"46": "กาฬสินธุ์",
	"47": "สกลนคร",
	"48": "นครพนม",
	"49": "มุกดาหาร",
	"50": "เชียงใหม่",
	"51": "ลำพูน",
	"52": "ลำปาง",
	"53": "อุตรดิตถ์",
	"54": "แพร่",
	"55": "น่าน",
	"56": "พะเยา",
	"57": "เชียงราย",
	"58": "แม่ฮ่องสอน",
	"60": "นครสวรรค์",
	"61": "อุทัยธานี",
	"62": "กำแพงเพชร",
	"63": "ตาก",
	"64": "สุโขทัย",
	"65": "พิษณุโลก",
	"66": "พิจิตร",
	"67": "เพชรบูรณ์",
	"70": "ราชบุรี",
	"71": "กาญจนบุรี",
	"72": "สุพรรณบุรี",
	"73": "นครปฐม",
	"74": "สมุทรสาคร",
	"75": "สมุทรสงคราม",
	"76": "เพชรบุรี",
	"77": "ประจวบคีรีขันธ์",
	"80": "นครศรีธรรมราช",
	"81": "กระบี่",
	"82": "พังงา",
	"83": "ภูเก็ต",
	"84": "สุราษฎร์ธานี",
	"85": "ระนอง",
	"86": "ชุมพร",
	"90": "สงขลา",
	"91": "สตูล",
	"92": "ตรัง",
	"93": "พัทลุง",
	"94": "ปัตตานี",
	"95": "ยะลา",
	"96": "นราธิวาส",
}

// officialSet contains fast lookup for official Thai names
var officialSet map[string]struct{}

func init() {
	officialSet = make(map[string]struct{}, len(official77Provinces))
	for _, p := range official77Provinces {
		officialSet[p] = struct{}{}
		aliasMap[strings.ToLower(p)] = p
	}
}

// AllProvinces returns a list of all 77 Thai provinces
func AllProvinces() []string {
	res := make([]string, len(official77Provinces))
	copy(res, official77Provinces)
	return res
}

// NormalizeProvince cleans up raw province strings, maps aliases/English names,
// falls back to postal code prefix, and returns UnknownProvince if still unresolved.
func NormalizeProvince(raw, postalCode string) string {
	cleanRaw := strings.TrimSpace(raw)
	// Remove common prefixes like "จ.", "จังหวัด"
	cleanRaw = strings.TrimPrefix(cleanRaw, "จ.")
	cleanRaw = strings.TrimPrefix(cleanRaw, "จังหวัด")
	cleanRaw = strings.TrimSpace(cleanRaw)

	if cleanRaw != "" {
		// 1. Direct official match
		if _, ok := officialSet[cleanRaw]; ok {
			return cleanRaw
		}

		// 2. Case-insensitive lookup in aliasMap
		lower := strings.ToLower(cleanRaw)
		if normalized, ok := aliasMap[lower]; ok {
			return normalized
		}

		// 3. Substring check: e.g. "Bangkok, Thailand" or "เมืองเชียงใหม่"
		for alias, standard := range aliasMap {
			if strings.Contains(lower, alias) {
				return standard
			}
		}
	}

	// 4. Postal code prefix fallback (first 2 digits)
	cleanPostal := strings.TrimSpace(postalCode)
	if len(cleanPostal) >= 2 {
		prefix := cleanPostal[:2]
		if provinceName, ok := postalCode2DigitPrefixMap[prefix]; ok {
			return provinceName
		}
	}

	return UnknownProvince
}
