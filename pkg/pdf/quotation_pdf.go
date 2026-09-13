package pdf

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type QuotationPDFData struct {
	Code            string
	Date            string
	ValidUntil      string
	LeadSource      string
	CompanyName     string
	CompanyAddress  string
	CompanyTaxID    string
	CompanyPhone    string
	CompanyEmail    string
	CompanyLogo     string
	CustomerName    string
	CustomerAddress string
	CustomerTaxID   string
	Lines           []QuotationPDFLine
	TotalAmount     float64
	VatRate         float64
	Note            string
	Status          string
}

type QuotationPDFLine struct {
	Index     int
	Name      string
	SKU       string
	Quantity  int
	UnitPrice float64
	LineTotal float64
}

// GenerateQuotationPDF formats an exact match to QuotationPrintTemplate.tsx in A4.
func GenerateQuotationPDF(data QuotationPDFData) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
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
		pdf.SetFillColor(243, 244, 246)
		pdf.SetDrawColor(209, 213, 219)
		pdf.SetLineWidth(0.2)
		pdf.RoundedRect(12, startY, logoW, logoH, 1.5, "1234", "FD")
		pdf.SetXY(12, startY+6)
		pdf.SetFont(fontName, "B", 8)
		pdf.SetTextColor(107, 114, 128)
		pdf.CellFormat(logoW, 6, "LOGO", "", 0, "C", false, 0, "")
	}

	textStartX := 12.0 + logoW + 3.5
	textWidth := 94.0

	pdf.SetXY(textStartX, startY)
	pdf.SetFont(fontName, "B", 13)
	pdf.SetTextColor(17, 24, 39)
	pdf.CellFormat(textWidth, 5.5, data.CompanyName, "", 1, "L", false, 0, "")

	pdf.SetX(textStartX)
	pdf.SetFont(fontName, "", 8)
	pdf.SetTextColor(55, 65, 81)
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

	// Right Header Box: ใบเสนอราคา
	pdf.SetXY(124, startY)
	pdf.SetFont(fontName, "B", 20)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(74, 8, "ใบเสนอราคา", "", 1, "R", false, 0, "")

	pdf.SetXY(124, pdf.GetY())
	pdf.SetFont(fontName, "B", 8)
	pdf.SetTextColor(107, 114, 128)
	pdf.CellFormat(74, 4, "QUOTATION · ต้นฉบับ", "", 1, "R", false, 0, "")

	// Meta grid (labels 22mm, values 52mm)
	metaY := pdf.GetY() + 2
	metaRows := [][]string{
		{"เลขที่", data.Code},
		{"วันที่", data.Date},
		{"ใช้ได้ถึง", data.ValidUntil},
		{"ช่องทาง", data.LeadSource},
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

	// Bottom border of Header
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
	pdf.SetTextColor(75, 85, 99)
	if data.CustomerAddress != "" {
		pdf.SetX(12)
		pdf.MultiCell(186, 4.2, data.CustomerAddress, "", "L", false)
	}
	if data.CustomerTaxID != "" {
		pdf.SetX(12)
		pdf.CellFormat(186, 4.2, "เลขประจำตัวผู้เสียภาษี "+data.CustomerTaxID, "", 1, "L", false, 0, "")
	}

	// Divider below customer
	pdf.SetDrawColor(209, 213, 219)
	pdf.SetLineWidth(0.3)
	pdf.Line(12, pdf.GetY()+3, 198, pdf.GetY()+3)
	pdf.SetY(pdf.GetY() + 6)

	// --- 3. ITEMS TABLE (5 Columns matching QuotationPrintTemplate) ---
	// Columns: # (12mm), Details (98mm), Qty (20mm), Unit Price (28mm), Total (28mm) = 186mm
	colW := []float64{12, 98, 20, 28, 28}
	headers := []string{"#", "รายละเอียด", "จำนวน", "ราคาต่อหน่วย", "มูลค่า"}
	aligns := []string{"C", "L", "R", "R", "R"}

	pdf.SetFillColor(243, 244, 246)
	pdf.SetTextColor(17, 24, 39)
	pdf.SetFont(fontName, "B", 8.5)
	pdf.SetDrawColor(107, 114, 128)
	pdf.SetLineWidth(0.5)

	tableHeadY := pdf.GetY()
	pdf.Line(12, tableHeadY, 198, tableHeadY)
	for i, h := range headers {
		pdf.CellFormat(colW[i], 8, h, "", 0, aligns[i], true, 0, "")
	}
	pdf.Ln(8)
	pdf.Line(12, pdf.GetY(), 198, pdf.GetY())

	// Table Body
	pdf.SetDrawColor(229, 231, 235)
	pdf.SetLineWidth(0.2)

	for i, line := range data.Lines {
		pdf.SetFont(fontName, "", 8.5)
		pdf.SetTextColor(31, 41, 55)

		pdf.CellFormat(colW[0], 7.5, fmt.Sprintf("%d", i+1), "B", 0, "C", false, 0, "")

		skuText := ""
		if line.SKU != "" && line.SKU != line.Name {
			skuText = " (" + line.SKU + ")"
		}
		pdf.SetFont(fontName, "B", 8.5)
		pdf.CellFormat(colW[1], 7.5, line.Name+skuText, "B", 0, "L", false, 0, "")

		pdf.SetFont(fontName, "", 8.5)
		pdf.CellFormat(colW[2], 7.5, fmt.Sprintf("%d", line.Quantity), "B", 0, "R", false, 0, "")
		pdf.CellFormat(colW[3], 7.5, formatBaht(line.UnitPrice), "B", 0, "R", false, 0, "")

		pdf.SetFont(fontName, "B", 8.5)
		pdf.CellFormat(colW[4], 7.5, formatBaht(line.LineTotal), "B", 1, "R", false, 0, "")
	}

	// --- 4. SUMMARY & REMARKS SECTION (Shifted down above signatures) ---
	summaryStartY := 205.0
	if pdf.GetY()+10 > summaryStartY {
		summaryStartY = pdf.GetY() + 10
	}

	// Left: หมายเหตุ / เงื่อนไข
	pdf.SetXY(12, summaryStartY)
	pdf.SetFont(fontName, "B", 9)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(100, 5, "หมายเหตุ / เงื่อนไข", "", 1, "L", false, 0, "")

	pdf.SetX(12)
	pdf.SetFont(fontName, "", 8.5)
	pdf.SetTextColor(75, 85, 99)
	if data.ValidUntil != "" {
		pdf.CellFormat(100, 4.5, fmt.Sprintf("ราคาดังกล่าวใช้ได้ถึง %s", data.ValidUntil), "", 1, "L", false, 0, "")
	}
	if data.Note != "" {
		pdf.SetX(12)
		pdf.MultiCell(100, 4.5, data.Note, "", "L", false)
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

	// Buyer / Customer
	pdf.SetXY(20, 245)
	pdf.CellFormat(sigW, 5, "ผู้อนุมัติสั่งซื้อ / ลูกค้า", "", 0, "C", false, 0, "")
	// Quoter
	pdf.SetXY(115, 245)
	pdf.CellFormat(sigW, 5, "ผู้เสนอราคา", "", 1, "C", false, 0, "")

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
