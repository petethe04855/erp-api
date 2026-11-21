package handler

import (
	"bytes"
	"testing"
)

func TestValidateSlipFile(t *testing.T) {
	realPDF := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<<>>\n%%EOF\n")
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00}

	cases := []struct {
		name     string
		filename string
		data     []byte
		wantMime string
		wantErr  string
	}{
		{"real pdf", "receipt.pdf", realPDF, "application/pdf", ""},
		{"real pdf upper ext", "RECEIPT.PDF", realPDF, "application/pdf", ""},
		{"fake pdf (jpeg bytes)", "receipt.pdf", jpegBytes, "", errSlipFakePDF},
		{"fake pdf (text)", "receipt.pdf", []byte("hello, not a pdf"), "", errSlipFakePDF},
		{"gif rejected", "slip.gif", []byte("GIF89a......"), "", errSlipBadType},
		{"no extension", "slip", jpegBytes, "", errSlipBadType},
		{"jpg", "slip.jpg", jpegBytes, "image/jpeg", ""},
		{"jpeg", "slip.jpeg", jpegBytes, "image/jpeg", ""},
		{"png", "slip.png", pngBytes, "image/png", ""},
		{"webp", "slip.webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "image/webp", ""},
		{"too large", "big.pdf", append(append([]byte{}, realPDF...), bytes.Repeat([]byte{0}, maxSlipFileSize)...), "", errSlipTooLarge},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mime, err := validateSlipFile(tc.filename, tc.data)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("validateSlipFile(%q) err = %v, want %q", tc.filename, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateSlipFile(%q) unexpected err: %v", tc.filename, err)
			}
			if mime != tc.wantMime {
				t.Fatalf("validateSlipFile(%q) mime = %q, want %q", tc.filename, mime, tc.wantMime)
			}
		})
	}
}
