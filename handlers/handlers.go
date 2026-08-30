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

func isNumericID(val string) bool {
	if val == "" {
		return false
	}
	_, err := strconv.ParseUint(val, 10, 64)
	return err == nil
}

// ByIDOrCode applies a query condition for an ID or Code parameter safely without PostgreSQL syntax errors (SQLSTATE 22P02).
func ByIDOrCode(db *gorm.DB, val string, codeColumn ...string) *gorm.DB {
	codeCol := "code"
	if len(codeColumn) > 0 && codeColumn[0] != "" {
		codeCol = codeColumn[0]
	}
	if isNumericID(val) {
		numID, _ := strconv.ParseUint(val, 10, 64)
		return db.Where(fmt.Sprintf("id = ? OR %s = ?", codeCol), numID, val)
	}
	return db.Where(fmt.Sprintf("%s = ?", codeCol), val)
}

// NotFound returns the standard API not-found response.
func NotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
}
