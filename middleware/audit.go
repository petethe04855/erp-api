package middleware

import (
	"fmt"
	"strings"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
)

// AuditMutations records successful API mutations so every module has a baseline audit trail.
// Business handlers may add a more specific event (for example approval or reversal).
func AuditMutations(c *fiber.Ctx) error {
	method := c.Method()
	if method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodPatch && method != fiber.MethodDelete {
		return c.Next()
	}

	path := c.Path()
	if strings.Contains(path, "/auth/") || strings.HasSuffix(path, "/stock-returns/:id/status") || strings.HasSuffix(path, "/credit-notes/:id/reverse") {
		return c.Next()
	}

	err := c.Next()
	if err != nil || c.Response().StatusCode() >= fiber.StatusBadRequest {
		return err
	}

	actor := "System"
	if name, ok := c.Locals("name").(string); ok && name != "" {
		actor = name
	} else if userID, ok := c.Locals("userID").(uint); ok {
		actor = fmt.Sprint(userID)
	}
	entity := strings.TrimPrefix(path, "/api/")
	if entity == "" {
		entity = path
	}
	_ = database.DB.Create(&models.AuditLog{
		Actor: actor, Action: method, Entity: entity, EntityID: c.Params("id"),
		After: fmt.Sprintf("HTTP %d", c.Response().StatusCode()), SourceRef: path,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}).Error
	return nil
}
