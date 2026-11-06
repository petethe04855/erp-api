package pdf

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// RenderCompanyLogo renders the company logo at (12, startY) with size 18x18mm.
// If loading fails or no logo is provided, it attempts fallback to the default
// Chawy logo, and if still unavailable, renders the placeholder LOGO box.
func RenderCompanyLogo(pdf *gofpdf.Fpdf, fontName string, startY float64, logoPath string) bool {
	logoW := 18.0
	logoH := 18.0

	resolvedPath, imageBytes := resolveLogoImage(logoPath)
	if resolvedPath != "" || len(imageBytes) > 0 {
		if renderLogoImage(pdf, startY, logoW, logoH, resolvedPath, imageBytes) {
			return true
		}
	}

	// Fallback to default chawy logo if custom logo failed or was empty
	if logoPath == "" || !strings.Contains(logoPath, "chawy.jpg") {
		resolvedFallback, fallbackBytes := resolveLogoImage("uploads/logos/chawy.jpg")
		if resolvedFallback != "" || len(fallbackBytes) > 0 {
			if renderLogoImage(pdf, startY, logoW, logoH, resolvedFallback, fallbackBytes) {
				return true
			}
		}
	}

	// Placeholder logo box matching InvoicePrintTemplate.tsx
	pdf.SetFillColor(243, 244, 246)
	pdf.SetDrawColor(209, 213, 219)
	pdf.SetLineWidth(0.2)
	pdf.RoundedRect(12, startY, logoW, logoH, 1.5, "1234", "FD")
	pdf.SetXY(12, startY+6)
	pdf.SetFont(fontName, "B", 8)
	pdf.SetTextColor(107, 114, 128)
	pdf.CellFormat(logoW, 6, "LOGO", "", 0, "C", false, 0, "")
	return false
}

func renderLogoImage(pdf *gofpdf.Fpdf, startY, logoW, logoH float64, localPath string, imgBytes []byte) bool {
	// 1. If we have a local file on disk, try direct gofpdf rendering
	if localPath != "" {
		imageType := detectImageType(localPath)
		if imageType != "" {
			pdf.ImageOptions(localPath, 12, startY, logoW, logoH, false, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, 0, "")
			if pdf.Error() == nil {
				return true
			}
			pdf.ClearError()
		}
	}

	// 2. Fallback: Decode with Go standard library and re-encode to clean JPEG
	var r io.Reader
	if len(imgBytes) > 0 {
		r = bytes.NewReader(imgBytes)
	} else if localPath != "" {
		f, err := os.Open(localPath)
		if err == nil {
			defer f.Close()
			r = f
		}
	}

	if r != nil {
		img, _, err := image.Decode(r)
		if err == nil {
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err == nil {
				imgName := fmt.Sprintf("logo_%x", md5.Sum(buf.Bytes()))
				pdf.RegisterImageOptionsReader(imgName, gofpdf.ImageOptions{ImageType: "JPG"}, &buf)
				if pdf.Error() == nil {
					pdf.ImageOptions(imgName, 12, startY, logoW, logoH, false, gofpdf.ImageOptions{ImageType: "JPG"}, 0, "")
					if pdf.Error() == nil {
						return true
					}
				}
				pdf.ClearError()
			}
		} else {
			log.Printf("[WARN] Failed to decode image for PDF logo: %v", err)
		}
	}

	return false
}

func detectImageType(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".png") {
		return "PNG"
	} else if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
		return "JPG"
	} else if strings.HasSuffix(lower, ".gif") {
		return "GIF"
	}
	return ""
}

func resolveLogoImage(logoPath string) (string, []byte) {
	clean := strings.TrimSpace(logoPath)
	if clean == "" {
		return "", nil
	}

	// If it's a URL with /uploads/, extract relative path first
	if idx := strings.Index(clean, "/uploads/"); idx != -1 {
		clean = clean[idx:]
	}
	clean = strings.TrimPrefix(clean, "/")

	// Check if it's a remote URL (http/https without /uploads/)
	if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(clean)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			data, readErr := io.ReadAll(resp.Body)
			if readErr == nil && len(data) > 0 {
				return "", data
			}
		}
	}

	baseName := filepath.Base(clean)
	var candidates []string

	candidates = append(candidates,
		clean,
		filepath.Join(".", clean),
		filepath.Join("uploads", "logos", baseName),
		filepath.Join("uploads", "images", baseName),
		filepath.Join("erp-api-v2", clean),
		filepath.Join("erp-api-v2", "uploads", "logos", baseName),
		filepath.Join("erp-api-v2", "uploads", "images", baseName),
		filepath.Join("..", clean),
		filepath.Join("..", "uploads", "logos", baseName),
		filepath.Join("..", "..", clean),
		filepath.Join("..", "..", "uploads", "logos", baseName),
	)

	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(execDir, clean),
			filepath.Join(execDir, "uploads", "logos", baseName),
			filepath.Join(execDir, "..", clean),
			filepath.Join(execDir, "..", "uploads", "logos", baseName),
		)
	}

	for _, cand := range candidates {
		if stat, err := os.Stat(cand); err == nil && !stat.IsDir() {
			return cand, nil
		}
	}

	return "", nil
}

// resolveLocalImagePath retains backwards compatibility for any external caller
func resolveLocalImagePath(logoPath string) string {
	path, _ := resolveLogoImage(logoPath)
	return path
}

