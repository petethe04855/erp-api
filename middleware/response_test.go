package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestStandardizeJSONResponseSuccess(t *testing.T) {
	app := fiber.New()
	app.Use(StandardizeJSONResponse)
	app.Get("/api/example", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"id": "A-1"})
	})

	res, err := app.Test(httptest.NewRequest("GET", "/api/example", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success || payload.Data.ID != "A-1" {
		t.Fatalf("unexpected response: %+v", payload)
	}
}

func TestStandardizeJSONResponseError(t *testing.T) {
	app := fiber.New()
	app.Use(StandardizeJSONResponse)
	app.Get("/api/example", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	})

	res, err := app.Test(httptest.NewRequest("GET", "/api/example", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var payload apiErrorResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Success || payload.Error.Code != "BAD_REQUEST" || payload.Error.Message != "Invalid input" {
		t.Fatalf("unexpected response: %+v", payload)
	}
}

func TestStandardizeJSONResponseLeavesFilesUnchanged(t *testing.T) {
	app := fiber.New()
	app.Use(StandardizeJSONResponse)
	app.Get("/api/export/example", func(c *fiber.Ctx) error {
		c.Type("xlsx")
		return c.SendString("file-content")
	})

	res, err := app.Test(httptest.NewRequest("GET", "/api/export/example", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.Header.Get("Content-Type") == "application/json" {
		t.Fatal("file response was converted to JSON")
	}
}
