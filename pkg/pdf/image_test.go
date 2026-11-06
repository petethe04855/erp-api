package pdf

import (
	"path/filepath"
	"testing"

	"github.com/jung-kurt/gofpdf"
)

func TestGofpdfImageLoad(t *testing.T) {
	files := []string{
		filepath.Join("..", "..", "uploads", "logos", "chawy.jpg"),
		filepath.Join("..", "..", "uploads", "logos", "1789554549-7ed8a20e.jpg"),
	}

	for _, f := range files {
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.ImageOptions(f, 10, 10, 20, 20, false, gofpdf.ImageOptions{ImageType: "JPG", ReadDpi: true}, 0, "")
		if err := pdf.Error(); err != nil {
			t.Errorf("Failed to load %s: %v", f, err)
		} else {
			t.Logf("Successfully loaded %s", f)
		}
	}
}

func TestRenderCompanyLogo(t *testing.T) {
	testCases := []struct {
		name     string
		logoPath string
	}{
		{"empty path fallback to chawy logo", ""},
		{"relative uploads path", "/uploads/logos/chawy.jpg"},
		{"direct filename", "chawy.jpg"},
		{"uploaded hash filename", "/uploads/logos/1789554549-7ed8a20e.jpg"},
		{"non-existent custom path fallback to chawy logo", "non_existent.png"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pdf := gofpdf.New("P", "mm", "A4", "")
			fontName := SetupPDFFonts(pdf)
			pdf.AddPage()
			rendered := RenderCompanyLogo(pdf, fontName, 15, tc.logoPath)
			if !rendered {
				t.Errorf("Expected logo to be rendered for case %s, but got false", tc.name)
			}
			if err := pdf.Error(); err != nil {
				t.Errorf("PDF encountered error: %v", err)
			}
		})
	}
}

func TestGenerateAllPDFsWithLogo(t *testing.T) {
	invData := InvoicePDFData{
		InvoiceNo:      "INV-001",
		CompanyName:    "Chawy Pet Food",
		CompanyAddress: "123 สุขุมวิท",
		CompanyTaxID:   "0123456789012",
		CompanyPhone:   "02-123-4567",
		CompanyEmail:   "hello@chawypet.com",
		CompanyLogo:    "/uploads/logos/chawy.jpg",
		CustomerName:   "Customer A",
		TotalAmount:    1000,
		Lines: []InvoicePDFLine{
			{Index: 1, Name: "Product A", Quantity: 1, UnitPrice: 1000, LineTotal: 1000},
		},
	}
	invBytes, err := GenerateInvoicePDF(invData)
	if err != nil || len(invBytes) == 0 {
		t.Fatalf("GenerateInvoicePDF failed: %v", err)
	}

	quoteData := QuotationPDFData{
		Code:           "QT-001",
		CompanyName:    "Chawy Pet Food",
		CompanyLogo:    "", // test empty fallback
		CustomerName:   "Customer B",
		TotalAmount:    1000,
		Lines: []QuotationPDFLine{
			{Index: 1, Name: "Product B", Quantity: 1, UnitPrice: 1000, LineTotal: 1000},
		},
	}
	quoteBytes, err := GenerateQuotationPDF(quoteData)
	if err != nil || len(quoteBytes) == 0 {
		t.Fatalf("GenerateQuotationPDF failed: %v", err)
	}

	soData := SalesOrderPDFData{
		Code:           "SO-001",
		CompanyName:    "Chawy Pet Food",
		CompanyLogo:    "/uploads/logos/1789554549-7ed8a20e.jpg",
		CustomerName:   "Customer C",
		TotalAmount:    1000,
		Lines: []SalesOrderPDFLine{
			{Index: 1, Name: "Product C", Quantity: 1, UnitPrice: 1000, LineTotal: 1000},
		},
	}
	soBytes, err := GenerateSalesOrderPDF(soData)
	if err != nil || len(soBytes) == 0 {
		t.Fatalf("GenerateSalesOrderPDF failed: %v", err)
	}
}
