package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestRequireRole(t *testing.T) {
	app := fiber.New()
	app.Get("/owner-only", func(c *fiber.Ctx) error {
		c.Locals("role", c.Get("X-Test-Role"))
		return c.Next()
	}, middleware.RequireRole("owner"), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Test authorized role (owner)
	req1 := httptest.NewRequest(http.MethodGet, "/owner-only", nil)
	req1.Header.Set("X-Test-Role", "owner")
	resp1, err := app.Test(req1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Role "admin" no longer exists in the system: an unknown role must be
	// rejected even when owner is required.
	reqAdmin := httptest.NewRequest(http.MethodGet, "/owner-only", nil)
	reqAdmin.Header.Set("X-Test-Role", "admin")
	respAdmin, err := app.Test(reqAdmin)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, respAdmin.StatusCode)

	// Test unauthorized role
	req2 := httptest.NewRequest(http.MethodGet, "/owner-only", nil)
	req2.Header.Set("X-Test-Role", "sales")
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp2.StatusCode)
}

func TestRequirePermission(t *testing.T) {
	app := fiber.New()
	app.Post("/delete-action", func(c *fiber.Ctx) error {
		c.Locals("role", c.Get("X-Test-Role"))
		return c.Next()
	}, middleware.RequirePermission("Delete"), func(c *fiber.Ctx) error {
		return c.SendString("deleted")
	})

	// Owner has Delete permission
	req1 := httptest.NewRequest(http.MethodPost, "/delete-action", nil)
	req1.Header.Set("X-Test-Role", "owner")
	resp1, err := app.Test(req1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	// Sales does not have Delete permission
	req2 := httptest.NewRequest(http.MethodPost, "/delete-action", nil)
	req2.Header.Set("X-Test-Role", "sales")
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp2.StatusCode)
}
