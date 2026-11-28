package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	domainPermission "chawy-erp-api/internal/domain/permission"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubChecker struct {
	allow map[string]bool // role|page|action
	err   error
}

func (s stubChecker) Can(_ context.Context, role, page string, action domainPermission.Action) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.allow[role+"|"+page+"|"+string(action)], nil
}

func runAccess(t *testing.T, role string, checker AccessChecker, rules ...Rule) int {
	t.Helper()
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		c.Locals("role", role)
		return c.Next()
	}, RequireAccess(checker, rules...), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	require.NoError(t, err)
	return resp.StatusCode
}

func TestRequireAccess(t *testing.T) {
	checker := stubChecker{allow: map[string]bool{"sales|sku|edit": true}}
	skuEdit := Rule{Page: "sku", Action: domainPermission.Edit}
	skuDelete := Rule{Page: "sku", Action: domainPermission.Delete}

	assert.Equal(t, fiber.StatusOK, runAccess(t, "sales", checker, skuEdit))
	assert.Equal(t, fiber.StatusForbidden, runAccess(t, "sales", checker, skuDelete))
	assert.Equal(t, fiber.StatusOK, runAccess(t, "sales", checker, skuDelete, skuEdit), "any one rule is enough")
	assert.Equal(t, fiber.StatusOK, runAccess(t, "owner", checker, skuDelete), "owner always passes")
}

func TestRequireAccessFailsClosed(t *testing.T) {
	rule := Rule{Page: "sku", Action: domainPermission.View}
	assert.Equal(t, fiber.StatusForbidden, runAccess(t, "sales", nil, rule))
	assert.Equal(t, fiber.StatusInternalServerError, runAccess(t, "sales", stubChecker{err: errors.New("db down")}, rule))
	assert.Equal(t, fiber.StatusOK, runAccess(t, "owner", nil, rule))
}
