package route

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"chawy-erp-api/internal/delivery/http/handler"
	domainPermission "chawy-erp-api/internal/domain/permission"
	usecasePermission "chawy-erp-api/internal/usecase/permission"
	"chawy-erp-api/pkg/jwt"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memPermissionRepo struct {
	rows []domainPermission.RolePermission
}

func (m *memPermissionRepo) ListAll(context.Context) ([]domainPermission.RolePermission, error) {
	return m.rows, nil
}

func (m *memPermissionRepo) SaveRole(_ context.Context, role string, rows []domainPermission.RolePermission) error {
	for _, r := range rows {
		r.Role = role
		m.rows = append(m.rows, r)
	}
	return nil
}

func newPermissionRouteApp(t *testing.T) *fiber.App {
	t.Helper()
	uc := usecasePermission.NewUsecase(&memPermissionRepo{})
	app := fiber.New()
	RegisterRoutes(Config{
		App:               app,
		FinanceHandler:    handler.NewFinanceHandler(stubFinanceUsecase{}, ""),
		PermissionHandler: handler.NewPermissionHandler(uc),
		AccessChecker:     uc,
		JWTSecret:         "test-secret",
	})
	return app
}

func callAs(t *testing.T, app *fiber.App, role, method, path, body string) (int, string) {
	t.Helper()
	token, err := jwt.GenerateToken(7, role+"@example.com", role, "test-secret", 1)
	require.NoError(t, err)
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestRolePermissionsPageIsOwnerOnly(t *testing.T) {
	app := newPermissionRouteApp(t)

	code, _ := callAs(t, app, "sales", "GET", "/api/v1/role-permissions/", "")
	assert.Equal(t, fiber.StatusForbidden, code)

	code, _ = callAs(t, app, "owner", "GET", "/api/v1/role-permissions/", "")
	assert.Equal(t, fiber.StatusOK, code)

	code, _ = callAs(t, app, "accountant", "PUT", "/api/v1/role-permissions/sales", `{"permissions":{"sku":{"view":true}}}`)
	assert.Equal(t, fiber.StatusForbidden, code)
}

func TestMatrixGuardsRoutesAndOwnerEditsApply(t *testing.T) {
	app := newPermissionRouteApp(t)

	// Finance expenses default to accountant only.
	code, _ := callAs(t, app, "sales", "GET", "/api/v1/finance/expenses", "")
	assert.Equal(t, fiber.StatusForbidden, code)
	code, _ = callAs(t, app, "sales", "DELETE", "/api/v1/expenses/1", "")
	assert.Equal(t, fiber.StatusForbidden, code)

	// The owner revokes SKU view for sales; /auth/permissions reflects it.
	code, _ = callAs(t, app, "owner", "PUT", "/api/v1/role-permissions/sales", `{"permissions":{"sku":{"view":false}}}`)
	require.Equal(t, fiber.StatusOK, code)

	code, body := callAs(t, app, "sales", "GET", "/api/v1/auth/permissions", "")
	require.Equal(t, fiber.StatusOK, code)
	var res struct {
		Data struct {
			Role        string                             `json:"role"`
			Permissions map[string]domainPermission.Access `json:"permissions"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &res))
	assert.Equal(t, "sales", res.Data.Role)
	assert.False(t, res.Data.Permissions["sku"].View)
	assert.True(t, res.Data.Permissions["customers"].Create, "untouched pages keep their defaults")
}
