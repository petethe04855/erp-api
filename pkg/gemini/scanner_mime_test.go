package gemini

import "testing"

func TestNormalizeMimeType(t *testing.T) {
	cases := map[string]string{
		"":                         "image/jpeg",
		"image/jpeg":               "image/jpeg",
		"image/jpg":                "image/jpeg",
		"IMAGE/PNG":                "image/png",
		"image/webp":               "image/webp",
		"application/pdf":          "application/pdf",
		"application/pdf; charset": "application/pdf",
		"image/gif":                "image/jpeg",
		"text/plain":               "image/jpeg",
	}
	for in, want := range cases {
		if got := normalizeMimeType(in); got != want {
			t.Errorf("normalizeMimeType(%q) = %q, want %q", in, got, want)
		}
	}
}
