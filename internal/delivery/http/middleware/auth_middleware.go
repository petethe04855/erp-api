package middleware

import (
	"context"
	"strings"

	"chawy-erp-api/pkg/jwt"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// UserStatusLoader lets the middleware check the user's current persisted
// state without depending on a concrete repository (FULL-03).
type UserStatusLoader interface {
	FindActiveUserRole(ctx context.Context, userID uint) (role string, active bool, err error)
}

// AuthMiddleware validates the JWT and then re-checks the user's current
// status and role in the database, so disabled accounts or demoted users lose
// access as soon as their row changes — not only when the token expires.
func AuthMiddleware(secret string, users UserStatusLoader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		path := c.Path()
		// Public paths that do not require JWT Authorization header
		if path == "/api/v1/health" ||
			path == "/api/v1/auth/login" ||
			path == "/api/v1/auth/register" ||
			strings.HasSuffix(path, "/callback") ||
			strings.HasSuffix(path, "/webhook") {
			return c.Next()
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Authorization header is required", "MISSING_TOKEN")
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return response.Unauthorized(c, "Invalid Authorization header format. Expected Bearer <token>", "INVALID_TOKEN_FORMAT")
		}

		claims, err := jwt.ValidateToken(parts[1], secret)
		if err != nil {
			return response.Unauthorized(c, "Invalid or expired token", "INVALID_TOKEN")
		}

		if users != nil {
			role, active, err := users.FindActiveUserRole(c.UserContext(), claims.UserID)
			if err != nil || !active {
				return response.Unauthorized(c, "Account is disabled or no longer exists", "ACCOUNT_DISABLED")
			}
			if !strings.EqualFold(role, claims.Role) {
				// Role changed since the token was issued: enforce current role.
				c.Locals("role", role)
			} else {
				c.Locals("role", claims.Role)
			}
		} else {
			c.Locals("role", claims.Role)
		}

		c.Locals("userID", claims.UserID)
		c.Locals("email", claims.Email)

		return c.Next()
	}
}

func RequireRole(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("role").(string)
		if !ok {
			return response.Forbidden(c, "Role information missing", "FORBIDDEN")
		}

		for _, role := range allowedRoles {
			if strings.EqualFold(userRole, role) {
				return c.Next()
			}
		}

		// Admin is granted access to any endpoint permitted for owner
		if strings.EqualFold(userRole, "admin") {
			for _, role := range allowedRoles {
				if strings.EqualFold(role, "owner") {
					return c.Next()
				}
			}
		}

		// Owner is granted access to any endpoint permitted for admin
		if strings.EqualFold(userRole, "owner") {
			for _, role := range allowedRoles {
				if strings.EqualFold(role, "admin") {
					return c.Next()
				}
			}
		}

		return response.Forbidden(c, "You do not have permission to perform this action", "FORBIDDEN")
	}
}

// PermissionMatrix is the central RBAC policy for API actions.
var PermissionMatrix = map[string][]string{
	"View":    {"owner", "admin", "sales", "warehouse", "accountant"},
	"Create":  {"owner", "admin", "sales", "warehouse", "accountant"},
	"Edit":    {"owner", "admin", "sales", "warehouse", "accountant"},
	"Delete":  {"owner", "admin", "accountant"},
	"Approve": {"owner", "admin", "accountant", "warehouse"},
	"Post":    {"owner", "admin", "accountant"},
	"Cancel":  {"owner", "admin", "accountant", "warehouse"},
	"Reverse": {"owner", "admin", "accountant"},
	"Export":  {"owner", "admin", "sales", "warehouse", "accountant"},
}

func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("role").(string)
		if !ok {
			return response.Forbidden(c, "Role information missing", "FORBIDDEN")
		}

		if strings.EqualFold(userRole, "owner") || strings.EqualFold(userRole, "admin") {
			return c.Next()
		}

		allowedRoles, exists := PermissionMatrix[permission]
		if !exists {
			return response.Forbidden(c, "Undefined permission", "FORBIDDEN")
		}

		for _, role := range allowedRoles {
			if strings.EqualFold(userRole, role) {
				return c.Next()
			}
		}

		return response.Forbidden(c, "You do not have permission to perform this action", "FORBIDDEN")
	}
}
