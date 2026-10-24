package gemini

import (
	"context"
	"os"
	"testing"
	"time"

	"chawy-erp-api/config"
)

func TestCleanJSONString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "```json\n{\"amount\": 150}\n```",
			expected: "{\"amount\": 150}",
		},
		{
			input:    "```\n{\"amount\": 250}\n```",
			expected: "{\"amount\": 250}",
		},
		{
			input:    "{\"amount\": 350}",
			expected: "{\"amount\": 350}",
		},
	}

	for _, tt := range tests {
		actual := cleanJSONString(tt.input)
		if actual != tt.expected {
			t.Errorf("cleanJSONString(%q) = %q; expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestNormalizeReceiptData(t *testing.T) {
	data := &ReceiptData{
		Amount:      100,
		Category:    "หมวดหมู่อื่นที่ไม่มีในระบบ",
		Channel:     "",
		ProductName: "",
		Vendor:      "Shell",
		Date:        "",
	}

	normalizeReceiptData(data)

	if data.Category != "ค่าใช้จ่ายในการบริหาร" {
		t.Errorf("expected default category 'ค่าใช้จ่ายในการบริหาร', got %q", data.Category)
	}
	if data.Channel != "ทั่วไป" {
		t.Errorf("expected default channel 'ทั่วไป', got %q", data.Channel)
	}
	if data.ProductName != "ค่าใช้จ่าย Shell" {
		t.Errorf("expected ProductName 'ค่าใช้จ่าย Shell', got %q", data.ProductName)
	}
	if data.Date == "" {
		t.Errorf("expected date not to be empty")
	}
}

func TestScanReceipt_MissingKey(t *testing.T) {
	s := NewScanner()
	_, err := s.ScanReceipt(context.Background(), "", []byte("fake"), "image/jpeg")
	if err == nil {
		t.Fatal("expected error when apiKey is empty, got nil")
	}
}

func TestScanUserUploadedSlip(t *testing.T) {
	imgPath := `C:\Users\pete\.gemini\antigravity-ide\brain\fb80fe34-b4ab-4563-bee9-38daf0395055\.user_uploaded\media_1792846770929.jpg`
	imgBytes, err := os.ReadFile(imgPath)
	if err != nil {
		t.Skipf("image file not found: %v", err)
	}

	cfg := config.LoadConfig()
	if cfg.GeminiAPIKey == "" {
		t.Skip("GEMINI_API_KEY is not set")
	}

	s := NewScanner()
	start := time.Now()
	res, err := s.ScanReceipt(context.Background(), cfg.GeminiAPIKey, imgBytes, "image/jpeg")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ScanReceipt failed: %v", err)
	}

	t.Logf("Scan took: %v", duration)
	t.Logf("Extracted Result: %+v", res)
}
