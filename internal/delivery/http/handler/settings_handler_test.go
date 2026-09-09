package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/internal/delivery/http/handler"
	domainSettings "chawy-erp-api/internal/domain/settings"
	usecaseSettings "chawy-erp-api/internal/usecase/settings"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type mockSettingsRepo struct {
	settings *domainSettings.Settings
}

func newMockSettingsRepo() *mockSettingsRepo {
	return &mockSettingsRepo{
		settings: &domainSettings.Settings{
			Company: domainSettings.CompanySettings{
				Name:          "Chawy Pet Food",
				TaxID:         "0123456789012",
				Address:       "Bangkok",
				Phone:         "02-123-4567",
				Email:         "hello@chawypet.com",
				Website:       "www.chawypet.com",
				Currency:      "THB",
				VatRate:       7,
				InvoicePrefix: "INV-2026-",
				SoPrefix:      "SO-2026-",
			},
			Notifications: domainSettings.NotificationSettings{
				NearExpiry: true,
			},
			Modules: domainSettings.ModuleSettings{
				Quotation: true,
			},
			LivePayroll: domainSettings.LivePayrollSettings{
				HourlyRate: 120,
			},
		},
	}
}

func (m *mockSettingsRepo) GetSettings(ctx context.Context) (*domainSettings.Settings, error) {
	return m.settings, nil
}

func (m *mockSettingsRepo) UpdateCompany(ctx context.Context, comp *domainSettings.CompanySettings) (*domainSettings.CompanySettings, error) {
	m.settings.Company = *comp
	return comp, nil
}

func (m *mockSettingsRepo) UpdateSettings(ctx context.Context, s *domainSettings.Settings) (*domainSettings.Settings, error) {
	m.settings = s
	return s, nil
}

func TestSettingsHandler_GetSettings(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := usecaseSettings.NewSettingsUsecase(repo)
	hdl := handler.NewSettingsHandler(uc)

	app := fiber.New()
	app.Get("/api/v1/settings", hdl.GetSettings)

	req := httptest.NewRequest("GET", "/api/v1/settings", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(t, err)
	assert.True(t, body["success"].(bool))

	data := body["data"].(map[string]interface{})
	company := data["company"].(map[string]interface{})
	assert.Equal(t, "Chawy Pet Food", company["name"])
	assert.Equal(t, "0123456789012", company["taxId"])
}

func TestSettingsHandler_UpdateSettings(t *testing.T) {
	repo := newMockSettingsRepo()
	uc := usecaseSettings.NewSettingsUsecase(repo)
	hdl := handler.NewSettingsHandler(uc)

	app := fiber.New()
	app.Put("/api/v1/settings", hdl.UpdateSettings)

	updatePayload := map[string]interface{}{
		"company": map[string]interface{}{
			"name":          "Chawy Updated Co., Ltd.",
			"taxId":         "9999999999999",
			"address":       "New Address",
			"phone":         "081-999-8888",
			"email":         "contact@chawy.com",
			"website":       "https://chawy.com",
			"currency":      "THB",
			"vatRate":       7,
			"invoicePrefix": "INV-2027-",
			"soPrefix":      "SO-2027-",
			"logoUrl":       "https://chawy.com/logo.png",
		},
	}
	payloadBytes, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest("PUT", "/api/v1/settings", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	assert.NoError(t, err)
	assert.True(t, body["success"].(bool))

	data := body["data"].(map[string]interface{})
	company := data["company"].(map[string]interface{})
	assert.Equal(t, "Chawy Updated Co., Ltd.", company["name"])
	assert.Equal(t, "9999999999999", company["taxId"])
}
