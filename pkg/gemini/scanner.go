package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ReceiptData represents structured data extracted from an expense receipt or slip.
type ReceiptData struct {
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
	Vendor      string  `json:"vendor"`
	InvoiceRef  string  `json:"invoice_ref"`
	ProductName string  `json:"product_name"`
	Category    string  `json:"category"`
	Channel     string  `json:"channel"`
	Description string  `json:"description"`
}

// AllowedCategories defines valid system expense categories.
var AllowedCategories = []string{
	"ค่าโฆษณา",
	"ค่าธรรมเนียมแพลตฟอร์ม",
	"ต้นทุนขาย/วัตถุดิบ",
	"ค่าใช้จ่ายในการบริหาร",
	"ค่าขนส่ง",
	"ค่าแรง/เงินเดือน",
	"อื่นๆ",
}

// Scanner handles receipt extraction via Google GenAI SDK.
type Scanner struct{}

// NewScanner creates a new Scanner instance.
func NewScanner() *Scanner {
	return &Scanner{}
}

const promptText = `You are an expert OCR and receipt/invoice/order/slip analysis system for Thai businesses.
Extract expense and transaction details from the provided image (receipt, tax invoice, bank transfer slip, or e-commerce purchase order screenshot such as Shopee, Lazada, or TikTok Shop).

Handling Multiple Items or Orders (สำคัญมาก):
- If the image contains multiple items or multiple purchase orders (e.g. 2 orders on a Shopee/Lazada screen):
  1. "amount": Calculate and SUM the total net amount of ALL visible items/orders together (e.g. 240 + 48 = 288). Do not miss any item.
  2. "product_name": Provide a concise combined summary of the items in Thai (e.g. "กล่องพัสดุ 2B และ สติกเกอร์เทปกระดาษ").
  3. "description": List ALL items with their quantity and individual prices clearly in Thai, formatted with bullet points:
     e.g. "1. กล่องพัสดุ เบอร์ 2B (20ใบ) x4 (฿240) | 2. สติกเกอร์เทปกระดาษ 50 หลา x1 (฿48)"
  4. "vendor": If multiple vendors/stores, join them with ' / ' (e.g. "ร้านเหมียวเลิฟเวอร์ Pet shop / Stickera4 Official Store TH").
  5. "channel": Identify the e-commerce platform if visible ('Shopee', 'TikTok', 'LINE', or 'ทั่วไป').

Return a JSON object with the following fields:
- "amount": (number) Total net amount to pay (Grand total / ยอดรวมสุทธิ). If multiple items/orders, SUM them together into one total amount. Must be a positive number.
- "date": (string) Transaction or delivery date in format YYYY-MM-DD. Convert Thai Buddhist Era (2567, 2568, 2569) to Christian Era (2024, 2025, 2026). If year is missing (e.g. 31 มี.ค.), assume year 2026.
- "vendor": (string) Name of store, vendor, payee, or billing company (if multiple, combine with ' / ').
- "invoice_ref": (string) Order number, tracking number, invoice number, or bank transaction reference.
- "product_name": (string) Concise summary of goods or services purchased in Thai.
- "category": (string) Must be strictly one of these 7 categories:
  ["ค่าโฆษณา", "ค่าธรรมเนียมแพลตฟอร์ม", "ต้นทุนขาย/วัตถุดิบ", "ค่าใช้จ่ายในการบริหาร", "ค่าขนส่ง", "ค่าแรง/เงินเดือน", "อื่นๆ"]
  Guidance:
  - Packaging (กล่องพัสดุ, เทป, ซอง, สติกเกอร์), raw materials, inventory -> "ต้นทุนขาย/วัตถุดิบ"
  - Fuel/gas, food/snacks, office supplies, utilities, repairs -> "ค่าใช้จ่ายในการบริหาร"
  - Shipping/courier/postal -> "ค่าขนส่ง"
  - Ads/Marketing -> "ค่าโฆษณา"
  - Platform fees -> "ค่าธรรมเนียมแพลตฟอร์ม"
  - Wage/Salary -> "ค่าแรง/เงินเดือน"
  - Default if unsure -> "ค่าใช้จ่ายในการบริหาร"
- "channel": (string) Channel name if identified (e.g. 'Shopee', 'TikTok', 'LINE', 'ทั่วไป'). Default is 'ทั่วไป'.
- "description": (string) Itemized list breakdown of all products with quantities and prices, plus any notes.

Return ONLY valid JSON matching this schema.`

// ScanReceipt calls Gemini Vision using official GenAI SDK.
func (s *Scanner) ScanReceipt(ctx context.Context, apiKey string, imageBytes []byte, mimeType string) (*ReceiptData, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("GEMINI_API_KEY is not configured")
	}

	if len(imageBytes) == 0 {
		return nil, errors.New("image bytes cannot be empty")
	}

	format := "jpeg"
	if strings.Contains(strings.ToLower(mimeType), "png") {
		format = "png"
	} else if strings.Contains(strings.ToLower(mimeType), "webp") {
		format = "webp"
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	models := []string{
		"gemini-3.5-flash-lite",
		"gemini-3.5-flash",
		"gemini-flash-latest",
		"gemini-3.8-flash",
	}

	var lastErr error
	for _, modelName := range models {
		model := client.GenerativeModel(modelName)
		model.ResponseMIMEType = "application/json"
		temp := float32(0.1)
		model.Temperature = &temp

		parts := []genai.Part{
			genai.ImageData(format, imageBytes),
			genai.Text(promptText),
		}

		resp, err := model.GenerateContent(ctx, parts...)
		if err != nil {
			lastErr = fmt.Errorf("gemini error (%s): %w", modelName, err)
			continue
		}

		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
			lastErr = fmt.Errorf("model %s returned empty response", modelName)
			continue
		}

		var rawText string
		for _, part := range resp.Candidates[0].Content.Parts {
			if t, ok := part.(genai.Text); ok {
				rawText += string(t)
			}
		}

		rawJSON := cleanJSONString(rawText)
		var result ReceiptData
		if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
			lastErr = fmt.Errorf("failed to parse receipt JSON (%s): %w (raw: %s)", modelName, err, rawJSON)
			continue
		}

		normalizeReceiptData(&result)
		return &result, nil
	}

	return nil, lastErr
}

// cleanJSONString removes markdown codeblock wrapper if present.
func cleanJSONString(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// normalizeReceiptData ensures category and date fields conform to expected formats.
func normalizeReceiptData(data *ReceiptData) {
	categoryFound := false
	for _, c := range AllowedCategories {
		if strings.TrimSpace(data.Category) == c {
			categoryFound = true
			data.Category = c
			break
		}
	}
	if !categoryFound {
		data.Category = "ค่าใช้จ่ายในการบริหาร"
	}

	if data.Channel == "" {
		data.Channel = "ทั่วไป"
	}

	if data.Date == "" {
		data.Date = time.Now().Format("2006-01-02")
	}

	if data.ProductName == "" {
		if data.Vendor != "" {
			data.ProductName = fmt.Sprintf("ค่าใช้จ่าย %s", data.Vendor)
		} else {
			data.ProductName = "ค่าใช้จ่ายทั่วไป"
		}
	}
}
