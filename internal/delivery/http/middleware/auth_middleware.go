package middleware

import (
	"strings"

	"chawy-erp-api/pkg/jwt"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware(secret string) fiber.Handler {
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

		c.Locals("userID", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)

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

// RequirePermission verifies the user's role against the central PermissionMatrix
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

