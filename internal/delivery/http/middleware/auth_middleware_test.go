package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/internal/delivery/http/middleware"
	"chawy-erp-api/pkg/jwt"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type mockUserStatusLoaderWithReset struct {
	role      string
	active    bool
	mustReset bool
}

func (m *mockUserStatusLoaderWithReset) FindActiveUserRole(ctx context.Context, userID uint) (string, bool, error) {
	return m.role, m.active, nil
}

func (m *mockUserStatusLoaderWithReset) FindActiveUserStatus(ctx context.Context, userID uint) (string, bool, bool, error) {
	return m.role, m.active, m.mustReset, nil
}

func TestAuthMiddleware_MustResetPasswordEnforcement(t *testing.T) {
	secret := "test-secret"
	token, _ := jwt.GenerateToken(1, "user@example.com", "sales", secret, 24)

	app := fiber.New()
	loader := &mockUserStatusLoaderWithReset{role: "sales", active: true, mustReset: true}
	api := app.Group("/api/v1", middleware.AuthMiddleware(secret, loader))
	api.Get("/orders", func(c *fiber.Ctx) error { return c.SendString("orders") })
	api.Post("/auth/change-password", func(c *fiber.Ctx) error { return c.SendString("password changed") })
	api.Get("/auth/me", func(c *fiber.Ctx) error { return c.SendString("me") })

	// Accessing /orders with mustReset = true should be 403 Forbidden
	reqOrders := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	reqOrders.Header.Set("Authorization", "Bearer "+token)
	respOrders, err := app.Test(reqOrders)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, respOrders.StatusCode)

	// Accessing /auth/change-password with mustReset = true should be allowed
	reqPass := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", nil)
	reqPass.Header.Set("Authorization", "Bearer "+token)
	respPass, err := app.Test(reqPass)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, respPass.StatusCode)

	// Accessing /auth/me with mustReset = true should be allowed
	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+token)
	respMe, err := app.Test(reqMe)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, respMe.StatusCode)
}


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
