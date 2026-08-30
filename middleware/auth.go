package middleware

import (
	"fmt"
	"os"
	"strings"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// PermissionMatrix is the central RBAC policy for API actions.
var PermissionMatrix = map[string][]string{
	"View":    {"owner", "sales", "warehouse", "accountant"},
	"Create":  {"owner", "sales", "warehouse", "accountant"},
	"Edit":    {"owner", "sales", "warehouse", "accountant"},
	"Delete":  {"owner", "accountant"},
	"Approve": {"owner", "accountant", "warehouse"},
	"Post":    {"owner", "accountant"},
	"Cancel":  {"owner", "accountant", "warehouse"},
	"Reverse": {"owner", "accountant"},
	"Export":  {"owner", "sales", "warehouse", "accountant"},
}

func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
		}
		for _, allowed := range PermissionMatrix[permission] {
			if role == allowed {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Permission denied", "permission": permission})
	}
}

// AuthRequired is a middleware that verifies JWT token in request headers
func AuthRequired(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing Authorization header",
		})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid Authorization header format. Must be 'Bearer <token>'",
		})
	}

	tokenStr := parts[1]
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Authentication is not configured"})
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid token claims",
		})
	}

	userIDVal := claims["userId"]
	var userIDValStr string
	switch v := userIDVal.(type) {
	case float64:
		userIDValStr = fmt.Sprintf("%.0f", v)
	case string:
		userIDValStr = v
	default:
		userIDValStr = fmt.Sprintf("%v", v)
	}

	if userIDValStr == "" || userIDValStr == "<nil>" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
	}

	var user models.AppUser
	if err := database.DB.Select("id", "role", "is_active").First(&user, "id = ?", userIDValStr).Error; err != nil || !user.IsActive {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Account is disabled or no longer exists"})
	}

	c.Locals("userID", user.ID)
	c.Locals("role", user.Role)

	return c.Next()
}

// RequireRoles restricts a route to one or more application roles.
func RequireRoles(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
		}
		if _, ok := allowed[role]; !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not have permission to perform this action"})
		}
		return c.Next()
	}
}
