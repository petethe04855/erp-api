package middleware

import (
	"crypto/subtle"
	"strings"

	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ServiceToken guards machine-to-machine routes (the CRM backoffice calling
// /api/v1/integrations/crm). The caller sends "Authorization: Bearer <token>";
// the token is compared in constant time. An empty configured token closes the
// routes (503) instead of leaving them open.
func ServiceToken(token string) fiber.Handler {
	want := []byte(token)
	return func(c *fiber.Ctx) error {
		if len(want) == 0 {
			return response.Error(c, fiber.StatusServiceUnavailable, "INTEGRATION_DISABLED",
				"CRM integration is not configured on this server")
		}
		auth := c.Get(fiber.HeaderAuthorization)
		got, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || got == "" {
			return response.Unauthorized(c, "Service token required", "MISSING_TOKEN")
		}
		if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			return response.Unauthorized(c, "Invalid service token", "INVALID_TOKEN")
		}
		return c.Next()
	}
}
