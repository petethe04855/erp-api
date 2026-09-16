package pdf

import (
	_ "embed"
	"os"

	"github.com/jung-kurt/gofpdf"
)

//go:embed fonts/tahoma.ttf
var tahomaFontRegular []byte

//go:embed fonts/tahomabd.ttf
var tahomaFontBold []byte

// SetupPDFFonts registers Thai-supported fonts into gofpdf.
// By embedding the fonts directly in the binary, it works across Windows and Linux (Render)
// without needing any fonts installed on the host OS.
func SetupPDFFonts(pdf *gofpdf.Fpdf) string {
	// 1. Check embedded fonts
	if len(tahomaFontRegular) > 0 {
		pdf.AddUTF8FontFromBytes("Tahoma", "", tahomaFontRegular)
		if len(tahomaFontBold) > 0 {
			pdf.AddUTF8FontFromBytes("Tahoma", "B", tahomaFontBold)
		} else {
			pdf.AddUTF8FontFromBytes("Tahoma", "B", tahomaFontRegular)
		}
		return "Tahoma"
	}

	// 2. Fallback to file paths if embed is somehow empty
	localCandidates := []string{
		"assets/fonts/tahoma.ttf",
		"pkg/pdf/fonts/tahoma.ttf",
		`C:\Windows\Fonts\tahoma.ttf`,
	}

	for _, cand := range localCandidates {
		if _, err := os.Stat(cand); err == nil {
			pdf.AddUTF8Font("Tahoma", "", cand)
			boldCand := cand
			if cand == `C:\Windows\Fonts\tahoma.ttf` {
				boldCand = `C:\Windows\Fonts\tahomabd.ttf`
			}
			if _, bErr := os.Stat(boldCand); bErr == nil {
				pdf.AddUTF8Font("Tahoma", "B", boldCand)
			} else {
				pdf.AddUTF8Font("Tahoma", "B", cand)
			}
			return "Tahoma"
		}
	}

	return "Helvetica"
}
