package route

import (
	"context"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/internal/delivery/http/handler"
	usecaseCRMIntegration "chawy-erp-api/internal/usecase/crmintegration"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type stubCRMIntegration struct {
	usecaseCRMIntegration.Usecase
}

func (stubCRMIntegration) Stock(context.Context, []uint, uint) ([]usecaseCRMIntegration.StockItem, error) {
	return []usecaseCRMIntegration.StockItem{{SKUID: 1, WarehouseID: 1, AvailableQty: 5}}, nil
}

// The CRM calls with a service token, not a staff JWT: the integration routes
// must answer before the protected group's AuthMiddleware sees the request.
func TestCRMIntegrationRoutesUseTheServiceToken(t *testing.T) {
	app := fiber.New()
	RegisterRoutes(Config{
		App:                   app,
		FinanceHandler:        handler.NewFinanceHandler(stubFinanceUsecase{}, ""),
		CRMIntegrationHandler: handler.NewCRMIntegrationHandler(stubCRMIntegration{}),
		CRMServiceToken:       "crm-service-token-for-tests-0123456789",
		JWTSecret:             "test-secret",
	})

	call := func(bearer string) int {
		req := httptest.NewRequest("GET", "/api/v1/integrations/crm/stock?skuIds=1", nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		return resp.StatusCode
	}
	require.Equal(t, 200, call("crm-service-token-for-tests-0123456789"))
	require.Equal(t, 401, call("not-the-token"))
	require.Equal(t, 401, call(""))
}
