package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Helper: Get audit trail standard time string.
func getNowStr() string {
	return time.Now().Format("2006-01-02T15:04")
}

// NextID generates next serial ID like prefix0001.
func NextID(db *gorm.DB, prefix string, model interface{}, idField string) (string, error) {
	var lastID string
	err := db.Model(model).Order(fmt.Sprintf("%s DESC", idField)).Limit(1).Pluck(idField, &lastID).Error
	if err != nil {
		return "", err
	}

	suffix := 1
	if lastID != "" && len(lastID) > len(prefix) {
		numPart := lastID[len(prefix):]
		if value, parseErr := strconv.Atoi(numPart); parseErr == nil {
			suffix = value + 1
		}
	}

	return fmt.Sprintf("%s%04d", prefix, suffix), nil
}

// NotFound returns the standard API not-found response.
func NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
}
