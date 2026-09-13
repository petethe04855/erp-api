package pdf

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type InvoicePDFData struct {
	InvoiceNo       string
	IssueDate       string
	DueDate         string
	OrderNo         string
	CompanyName     string
	CompanyAddress  string
	CompanyTaxID    string
	CompanyPhone    string
	CompanyEmail    string
	CompanyLogo     string
	CustomerName    string
	CustomerAddress string
	CustomerTaxID   string
	CustomerBranch  string
	Lines           []InvoicePDFLine
	TotalAmount     float64
	VatRate         float64
	PaymentMethod   string
	Status          string
}

type InvoicePDFLine struct {
	Index     int
	Name      string
	SKU       string
	Quantity  int
	Unit      string
	UnitPrice float64
	Discount  float64
	LineTotal float64
}

// GenerateInvoicePDF formats an exact match to InvoicePrintTemplate.tsx in A4.
func GenerateInvoicePDF(data InvoicePDFData) ([]byte, error) {
	// A4 = 210mm x 297mm
	pdf := gofpdf.New("P", "mm", "A4", "")
	// px-10 in 794px canvas is approx 10.5mm margin
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 12)

	fontName := "Helvetica"
	tahomaPath := `C:\Windows\Fonts\tahoma.ttf`
	if _, err := os.Stat(tahomaPath); err == nil {
		pdf.AddUTF8Font("Tahoma", "", tahomaPath)
		tahomaBoldPath := `C:\Windows\Fonts\tahomabd.ttf`
		if _, err := os.Stat(tahomaBoldPath); err == nil {
			pdf.AddUTF8Font("Tahoma", "B", tahomaBoldPath)
		} else {
			pdf.AddUTF8Font("Tahoma", "B", tahomaPath)
		}
		fontName = "Tahoma"
	}

	pdf.AddPage()

	// --- 1. HEADER SECTION ---
	startY := pdf.GetY()

	// Left: Company Logo (18x18mm) + Company Info beside it
	logoW := 18.0
	logoH := 18.0
	logoRendered := false

	if data.CompanyLogo != "" {
		logoTarget := resolveLocalImagePath(data.CompanyLogo)
		if logoTarget != "" {
			// Register and render image
			imageType := ""
			lower := strings.ToLower(logoTarget)
			if strings.HasSuffix(lower, ".png") {
				imageType = "PNG"
			} else if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
				imageType = "JPG"
			}
			pdf.ImageOptions(logoTarget, 12, startY, logoW, logoH, false, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, 0, "")
			if pdf.Error() == nil {
				logoRendered = true
			} else {
				pdf.ClearError()
			}
		}
	}

	if !logoRendered {
		// Placeholder logo box matching InvoicePrintTemplate.tsx
		pdf.SetFillColor(243, 244, 246)
		pdf.SetDrawColor(209, 213, 219)
		pdf.SetLineWidth(0.2)
		pdf.RoundedRect(12, startY, logoW, logoH, 1.5, "1234", "FD")
		pdf.SetXY(12, startY+6)
		pdf.SetFont(fontName, "B", 8)
		pdf.SetTextColor(107, 114, 128)
		pdf.CellFormat(logoW, 6, "LOGO", "", 0, "C", false, 0, "")
	}

	// Company Info Left Box beside logo (starts at X=34, width ~88mm)
	textStartX := 12.0 + logoW + 3.5
	textWidth := 94.0

	pdf.SetXY(textStartX, startY)
	pdf.SetFont(fontName, "B", 13)
	pdf.SetTextColor(17, 24, 39) // text-gray-900
	pdf.CellFormat(textWidth, 5.5, data.CompanyName, "", 1, "L", false, 0, "")

	pdf.SetX(textStartX)
	pdf.SetFont(fontName, "", 8)
	pdf.SetTextColor(55, 65, 81) // text-gray-700
	if data.CompanyAddress != "" {
		pdf.MultiCell(textWidth, 3.8, data.CompanyAddress, "", "L", false)
	}
	if data.CompanyTaxID != "" && data.CompanyTaxID != "–" {
		pdf.SetX(textStartX)
		pdf.CellFormat(textWidth, 3.8, fmt.Sprintf("เลขประจำตัวผู้เสียภาษี %s", data.CompanyTaxID), "", 1, "L", false, 0, "")
	}
	contactLine := ""
	if data.CompanyPhone != "" && data.CompanyPhone != "–" {
		contactLine = data.CompanyPhone
	}
	if data.CompanyEmail != "" && data.CompanyEmail != "–" {
		if contactLine != "" {
			contactLine += " · " + data.CompanyEmail
		} else {
			contactLine = data.CompanyEmail
		}
	}
	if contactLine != "" {
		pdf.SetX(textStartX)
		pdf.CellFormat(textWidth, 3.8, contactLine, "", 1, "L", false, 0, "")
	}
	leftEndY := pdf.GetY()
	if leftEndY < startY+logoH {
		leftEndY = startY + logoH
	}

	// Right Header Box (width ~74mm)
	pdf.SetXY(124, startY)
	pdf.SetFont(fontName, "B", 20)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(74, 8, "ใบแจ้งหนี้", "", 1, "R", false, 0, "")

	pdf.SetXY(124, pdf.GetY())
	pdf.SetFont(fontName, "B", 8)
	pdf.SetTextColor(107, 114, 128) // text-gray-500
	pdf.CellFormat(74, 4, "INVOICE · ต้นฉบับ", "", 1, "R", false, 0, "")

	// Meta grid (labels 22mm, values 52mm)
	metaY := pdf.GetY() + 2
	metaRows := [][]string{
		{"เลขที่", data.InvoiceNo},
		{"วันที่", data.IssueDate},
		{"ครบกำหนด", data.DueDate},
		{"ใบสั่งขาย", data.OrderNo},
	}

	for _, row := range metaRows {
		pdf.SetXY(124, metaY)
		pdf.SetFont(fontName, "", 8.5)
		pdf.SetTextColor(0, 0, 0)
		pdf.CellFormat(22, 4.5, row[0], "", 0, "L", false, 0, "")

		pdf.SetFont(fontName, "B", 8.5)
		pdf.SetTextColor(31, 41, 55)
		val := row[1]
		if val == "" {
			val = "–"
		}
		pdf.CellFormat(52, 4.5, val, "", 1, "R", false, 0, "")
		metaY += 4.5
	}

	headerMaxY := leftEndY
	if metaY > headerMaxY {
		headerMaxY = metaY
	}

	// Bottom border of Header (border-b-2 border-gray-800)
	pdf.SetDrawColor(31, 41, 55)
	pdf.SetLineWidth(0.6)
	pdf.Line(12, headerMaxY+3, 198, headerMaxY+3)

	// --- 2. CUSTOMER SECTION ---
	custY := headerMaxY + 7
	pdf.SetXY(12, custY)
	pdf.SetFont(fontName, "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(186, 4.5, "ลูกค้า", "", 1, "L", false, 0, "")

	pdf.SetX(12)
	pdf.SetFont(fontName, "B", 10.5)
	pdf.SetTextColor(17, 24, 39)
	pdf.CellFormat(186, 5.5, data.CustomerName, "", 1, "L", false, 0, "")

	pdf.SetFont(fontName, "", 8.5)
	pdf.SetTextColor(75, 85, 99) // text-gray-600
	if data.CustomerAddress != "" {
		pdf.SetX(12)
		pdf.MultiCell(186, 4.2, data.CustomerAddress, "", "L", false)
	}

	custTax := ""
	if data.CustomerTaxID != "" {
		custTax = "เลขประจำตัวผู้เสียภาษี " + data.CustomerTaxID
	}
	if data.CustomerBranch != "" {
		if custTax != "" {
			custTax += " · สาขา " + data.CustomerBranch
		} else {
			custTax = "สาขา " + data.CustomerBranch
		}
	}
	if custTax != "" {
		pdf.SetX(12)
		pdf.CellFormat(186, 4.2, custTax, "", 1, "L", false, 0, "")
	}

	// Divider below customer
	pdf.SetDrawColor(209, 213, 219) // border-gray-300
	pdf.SetLineWidth(0.3)
	pdf.Line(12, pdf.GetY()+3, 198, pdf.GetY()+3)
	pdf.SetY(pdf.GetY() + 6)

	// --- 3. ITEMS TABLE ---
	// Columns: # (10mm), Details (80mm), Qty (14mm), Unit (14mm), Unit Price (22mm), Discount (20mm), Total (26mm) = 186mm
	colW := []float64{10, 80, 14, 14, 22, 20, 26}
	headers := []string{"#", "รายละเอียด", "จำนวน", "หน่วย", "ราคาต่อหน่วย", "ส่วนลด", "มูลค่า"}
	aligns := []string{"C", "L", "R", "C", "R", "R", "R"}

	// Table Header background bg-gray-100, border-y-2 border-gray-500
	pdf.SetFillColor(243, 244, 246)
	pdf.SetTextColor(17, 24, 39)
	pdf.SetFont(fontName, "B", 8.5)
	pdf.SetDrawColor(107, 114, 128)
	pdf.SetLineWidth(0.5)

	tableHeadY := pdf.GetY()
	// Draw top and bottom border
	pdf.Line(12, tableHeadY, 198, tableHeadY)
	for i, h := range headers {
		pdf.CellFormat(colW[i], 8, h, "", 0, aligns[i], true, 0, "")
	}
	pdf.Ln(8)
	pdf.Line(12, pdf.GetY(), 198, pdf.GetY())

	// Table Body
	pdf.SetDrawColor(229, 231, 235) // border-gray-200
	pdf.SetLineWidth(0.2)

	for i, line := range data.Lines {
		pdf.SetFont(fontName, "", 8.5)
		pdf.SetTextColor(31, 41, 55)

		currY := pdf.GetY()
		pdf.CellFormat(colW[0], 7.5, fmt.Sprintf("%d", i+1), "B", 0, "C", false, 0, "")

		// Name + SKU
		skuText := ""
		if line.SKU != "" && line.SKU != line.Name {
			skuText = " (" + line.SKU + ")"
		}
		pdf.SetFont(fontName, "B", 8.5)
		pdf.CellFormat(colW[1], 7.5, line.Name+skuText, "B", 0, "L", false, 0, "")

		pdf.SetFont(fontName, "", 8.5)
		pdf.CellFormat(colW[2], 7.5, fmt.Sprintf("%d", line.Quantity), "B", 0, "R", false, 0, "")
		unit := line.Unit
		if unit == "" {
			unit = "ชิ้น"
		}
		pdf.CellFormat(colW[3], 7.5, unit, "B", 0, "C", false, 0, "")
		pdf.CellFormat(colW[4], 7.5, formatBaht(line.UnitPrice), "B", 0, "R", false, 0, "")
		pdf.CellFormat(colW[5], 7.5, formatBaht(line.Discount), "B", 0, "R", false, 0, "")

		pdf.SetFont(fontName, "B", 8.5)
		pdf.CellFormat(colW[6], 7.5, formatBaht(line.LineTotal), "B", 1, "R", false, 0, "")
		_ = currY
	}

	// --- 4. SUMMARY & REMARKS SECTION ---
	// จัดตำแหน่งบล็อกสรุปยอดเงินและหมายเหตุให้อยู่ลงมาด้านล่างเหนือโซนลายเซ็น
	summaryStartY := 205.0
	if pdf.GetY()+10 > summaryStartY {
		summaryStartY = pdf.GetY() + 10
	}

	// Left: หมายเหตุ
	pdf.SetXY(12, summaryStartY)
	pdf.SetFont(fontName, "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(100, 5, "หมายเหตุ", "", 1, "L", false, 0, "")

	pdf.SetX(12)
	pdf.SetFont(fontName, "", 8.5)
	pdf.SetTextColor(75, 85, 99)
	if data.DueDate != "" {
		pdf.CellFormat(100, 4.5, fmt.Sprintf("ครบกำหนดชำระ %s", data.DueDate), "", 1, "L", false, 0, "")
	}
	if data.PaymentMethod != "" {
		pdf.SetX(12)
		pdf.CellFormat(100, 4.5, fmt.Sprintf("วิธีชำระเงิน %s", data.PaymentMethod), "", 1, "L", false, 0, "")
	}

	// Right: Financial breakdown
	vatRate := data.VatRate
	if vatRate <= 0 {
		vatRate = 7.0
	}
	total := data.TotalAmount
	beforeVat := total / (1.0 + vatRate/100.0)
	vat := total - beforeVat

	calcX := 115.0
	lblW := 45.0
	valW := 38.0

	// Top border over totals
	pdf.SetDrawColor(17, 24, 39)
	pdf.SetLineWidth(0.6)
	pdf.Line(calcX, summaryStartY, 198, summaryStartY)

	pdf.SetXY(calcX, summaryStartY+2)
	pdf.SetFont(fontName, "", 8.5)
	pdf.SetTextColor(31, 41, 55)
	pdf.CellFormat(lblW, 5, "ราคาสินค้าก่อนภาษี", "", 0, "L", false, 0, "")
	pdf.CellFormat(valW, 5, formatBaht(beforeVat), "", 1, "R", false, 0, "")

	pdf.SetX(calcX)
	pdf.CellFormat(lblW, 5, fmt.Sprintf("ภาษีมูลค่าเพิ่ม %.0f%%", vatRate), "", 0, "L", false, 0, "")
	pdf.CellFormat(valW, 5, formatBaht(vat), "", 1, "R", false, 0, "")

	// Border above total amount
	totalLineY := pdf.GetY() + 1
	pdf.Line(calcX, totalLineY, 198, totalLineY)

	pdf.SetXY(calcX, totalLineY+1)
	pdf.SetFont(fontName, "B", 10)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(lblW, 7, "จำนวนเงินรวมทั้งสิ้น", "", 0, "L", false, 0, "")
	pdf.CellFormat(valW, 7, formatBaht(total), "", 1, "R", false, 0, "")

	// Thai Baht Text
	pdf.SetXY(calcX, pdf.GetY())
	pdf.SetFont(fontName, "B", 7.5)
	pdf.SetTextColor(55, 65, 81)
	pdf.CellFormat(lblW+valW, 4.5, fmt.Sprintf("(%s)", toBahtText(total)), "", 1, "C", false, 0, "")

	// --- 5. SIGNATURE FOOTER ---
	pdf.SetY(250)
	sigW := 75.0

	pdf.SetFont(fontName, "", 8.5)
	pdf.SetTextColor(31, 41, 55)

	// Receiver
	pdf.SetXY(20, 245)
	pdf.CellFormat(sigW, 5, "ผู้รับสินค้า / ผู้ซื้อ", "", 0, "C", false, 0, "")
	// Approver
	pdf.SetXY(115, 245)
	pdf.CellFormat(sigW, 5, "ผู้อนุมัติ / ผู้ขาย", "", 1, "C", false, 0, "")

	// Line
	pdf.SetDrawColor(156, 163, 175)
	pdf.SetLineWidth(0.3)
	pdf.Line(25, 262, 25+65, 262)
	pdf.Line(120, 262, 120+65, 262)

	pdf.SetXY(20, 264)
	pdf.CellFormat(sigW, 5, "วันที่ _____ / _____ / _________", "", 0, "C", false, 0, "")
	pdf.SetXY(115, 264)
	pdf.CellFormat(sigW, 5, "วันที่ _____ / _____ / _________", "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	err := pdf.Output(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatBaht(v float64) string {
	return fmt.Sprintf("฿%.2f", v)
}

// toBahtText converts numbers to Thai baht text equivalent to bahtText() in printUtils.ts
func toBahtText(num float64) string {
	if math.IsNaN(num) || num == 0 {
		return "ศูนย์บาทถ้วน"
	}
	digits := []string{"ศูนย์", "หนึ่ง", "สอง", "สาม", "สี่", "ห้า", "หก", "เจ็ด", "แปด", "เก้า"}
	places := []string{"", "สิบ", "ร้อย", "พัน", "หมื่น", "แสน"}

	var thaiInt func(n int64, isMillion bool) string
	thaiInt = func(n int64, isMillion bool) string {
		if n == 0 {
			if isMillion {
				return ""
			}
			return digits[0]
		}
		if n >= 1_000_000 {
			mil := n / 1_000_000
			rem := n % 1_000_000
			return thaiInt(mil, false) + "ล้าน" + thaiInt(rem, true)
		}
		s := fmt.Sprintf("%d", n)
		lenS := len(s)
		var res strings.Builder
		for i, r := range s {
			d := int(r - '0')
			if d == 0 {
				continue
			}
			pl := lenS - i - 1
			if pl == 1 && d == 1 {
				res.WriteString("สิบ")
				continue
			}
			if pl == 1 && d == 2 {
				res.WriteString("ยี่สิบ")
				continue
			}
			if pl == 0 && d == 1 && lenS > 1 && s[lenS-2] != '0' {
				res.WriteString("เอ็ด")
				continue
			}
			res.WriteString(digits[d] + places[pl])
		}
		return res.String()
	}

	absNum := math.Abs(num)
	baht := int64(math.Floor(absNum))
	satang := int64(math.Round((absNum - float64(baht)) * 100))

	prefix := ""
	if num < 0 {
		prefix = "ลบ"
	}
	bahtPart := ""
	if baht > 0 {
		bahtPart = thaiInt(baht, false) + "บาท"
	}
	satangPart := "ถ้วน"
	if satang > 0 {
		satangPart = thaiInt(satang, false) + "สตางค์"
	}

	return prefix + bahtPart + satangPart
}

// resolveLocalImagePath checks various possible locations for an image path
func resolveLocalImagePath(logoPath string) string {
	clean := strings.TrimSpace(logoPath)
	if clean == "" {
		return ""
	}
	// If it's a full URL like http://.../uploads/images/abc.png, extract the relative path
	if idx := strings.Index(clean, "/uploads/"); idx != -1 {
		clean = clean[idx:]
	}
	clean = strings.TrimPrefix(clean, "/")

	candidates := []string{
		clean,
		filepath.Join(".", clean),
		filepath.Join("uploads", "images", filepath.Base(clean)),
		filepath.Join("erp-api", clean),
	}

	for _, cand := range candidates {
		if stat, err := os.Stat(cand); err == nil && !stat.IsDir() {
			return cand
		}
	}
	return ""
}
