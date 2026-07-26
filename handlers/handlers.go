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

// NextCode generates next serial code like prefix0001.
func NextCode(db *gorm.DB, prefix string, model interface{}, codeField string) (string, error) {
	var lastCode string
	err := db.Model(model).Order(fmt.Sprintf("%s DESC", codeField)).Limit(1).Pluck(codeField, &lastCode).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return "", err
	}

	suffix := 1
	if lastCode != "" && len(lastCode) >= len(prefix) {
		numPart := lastCode[len(prefix):]
		if value, parseErr := strconv.Atoi(numPart); parseErr == nil {
			suffix = value + 1
		}
	}

	return fmt.Sprintf("%s%04d", prefix, suffix), nil
}

// NextID generates next serial ID like prefix0001 (Legacy compatibility helper).
func NextID(db *gorm.DB, prefix string, model interface{}, idField string) (string, error) {
	return NextCode(db, prefix, model, idField)
}

// NotFound returns the standard API not-found response.
func NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
}
