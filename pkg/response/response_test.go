package response_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

func TestResponse_OK(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return response.OK(c, map[string]string{"foo": "bar"}, "Operation successful")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var res response.SuccessResponse
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success to be true")
	}
	if res.Message != "Operation successful" {
		t.Errorf("expected message 'Operation successful', got '%s'", res.Message)
	}
}

func TestResponse_Error(t *testing.T) {
	app := fiber.New()
	app.Get("/error", func(c *fiber.Ctx) error {
		return response.NotFound(c, "Item not found", "ITEM_NOT_FOUND")
	})

	req := httptest.NewRequest("GET", "/error", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var res response.ErrorResponse
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if res.Success {
		t.Errorf("expected success to be false")
	}
	if res.Error.Code != "ITEM_NOT_FOUND" {
		t.Errorf("expected error code 'ITEM_NOT_FOUND', got '%s'", res.Error.Code)
	}
}
