package route

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"chawy-erp-api/internal/delivery/http/handler"
	domainFinance "chawy-erp-api/internal/domain/finance"
	usecaseFinance "chawy-erp-api/internal/usecase/finance"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFinanceUsecase only implements CreateIncome; embedding the interface
// means any other method panics if a test accidentally reaches it.
type stubFinanceUsecase struct {
	usecaseFinance.Usecase
}

func (stubFinanceUsecase) CreateIncome(ctx context.Context, in usecaseFinance.CreateIncomeInput) (*domainFinance.Income, error) {
	return &domainFinance.Income{ID: 1, Code: "INC-2026-0001", Amount: in.Amount}, nil
}

func newPublicRouteTestApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	RegisterRoutes(Config{
		App:            app,
		FinanceHandler: handler.NewFinanceHandler(stubFinanceUsecase{}, ""),
		JWTSecret:      "test-secret",
	})
	return app
}

// The LINE OA / LIFF upload page has no JWT, so the income endpoints must be
// reachable through the /api/v1/public/ prefix without an Authorization header.
func TestPublicIncomeRoutesDoNotRequireAuth(t *testing.T) {
	app := newPublicRouteTestApp(t)

	t.Run("POST /public/incomes", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/public/incomes", strings.NewReader(`{"amount":100}`))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.NotEqual(t, fiber.StatusUnauthorized, resp.StatusCode)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	t.Run("POST /public/incomes/scan-slip", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/public/incomes/scan-slip", nil)

		resp, err := app.Test(req)
		require.NoError(t, err)
		// No multipart body here, so the handler answers 400 — what matters is
		// that auth did not reject the request with 401.
		assert.NotEqual(t, fiber.StatusUnauthorized, resp.StatusCode)
	})
}
