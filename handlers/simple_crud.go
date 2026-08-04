package handlers

import (
	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func updateSimple(c *fiber.Ctx, value interface{}, column, id string) error {
	if err := database.DB.First(value, column+" = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := c.BodyParser(value); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := database.DB.Save(value).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(value)
}

func deleteSimple(c *fiber.Ctx, value interface{}, column, id string) error {
	result := database.DB.Where(column+" = ?", id).Delete(value)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
	}
	return c.JSON(fiber.Map{"deleted": true, "id": id})
}

func UpdateExpense(c *fiber.Ctx) error {
	return rejectPostedExpenseMutation(c)
}
func DeleteExpense(c *fiber.Ctx) error {
	return rejectPostedExpenseMutation(c)
}

func rejectPostedExpenseMutation(c *fiber.Ctx) error {
	var expense models.Expense
	if err := database.DB.First(&expense, "id = ?", c.Params("id")).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Expense not found"})
	}
	var count int64
	database.DB.Model(&models.JournalEntry{}).Where("source_type = ? AND source_id = ?", "expense", expense.ID).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "posted expense is immutable; create a reversal instead"})
	}
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "expense mutation is disabled"})
}
func UpdateBudget(c *fiber.Ctx) error {
	return updateSimple(c, &models.MonthBudget{}, "id", c.Params("id"))
}
func DeleteBudget(c *fiber.Ctx) error {
	return deleteSimple(c, &models.MonthBudget{}, "id", c.Params("id"))
}
func UpdateManualOrder(c *fiber.Ctx) error {
	return updateSimple(c, &models.ManualOrder{}, "id", c.Params("id"))
}
func DeleteManualOrder(c *fiber.Ctx) error {
	return deleteSimple(c, &models.ManualOrder{}, "id", c.Params("id"))
}
func UpdateContentSchedule(c *fiber.Ctx) error {
	return updateSimple(c, &models.ContentScheduleItem{}, "id", c.Params("id"))
}
func DeleteContentSchedule(c *fiber.Ctx) error {
	return deleteSimple(c, &models.ContentScheduleItem{}, "id", c.Params("id"))
}
