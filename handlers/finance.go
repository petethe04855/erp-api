package handlers

import (
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// POST /api/expenses
func CreateExpense(c *fiber.Ctx) error {
	var exp models.Expense
	if err := c.BodyParser(&exp); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "EXP-2026-", &models.Expense{}, "code")
		if err != nil {
			return err
		}
		exp.Code = code
		if exp.Date == "" {
			exp.Date = time.Now().Format("2006-01-02")
		}
		exp.CreatedBy = username.(string)

		if err := tx.Create(&exp).Error; err != nil {
			return err
		}
		if exp.Amount <= 0 {
			return fiber.NewError(fiber.StatusBadRequest, "expense amount must be greater than zero")
		}
		_, err = postJournal(tx, postingRequest{
			Date: exp.Date, SourceType: "expense", SourceID: exp.ID, SourceRef: exp.Code,
			Description: exp.Description, CreatedBy: username.(string),
			Lines: []postingLine{{AccountCode: accountOperatingExpense, Debit: exp.Amount, Channel: exp.Channel}, {AccountCode: accountCash, Credit: exp.Amount, Channel: exp.Channel}},
		})
		return err
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(exp)
}

// POST /api/budgets
func UpsertBudget(c *fiber.Ctx) error {
	var req models.MonthBudget
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.Year < 2000 || req.Month < 1 || req.Month > 12 || req.BudgetAmount < 0 || req.Category == "" || req.Channel == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "budget requires valid year, month, category, channel, and non-negative amount"})
	}

	var budget models.MonthBudget
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("year = ? AND month = ? AND category = ? AND channel = ?",
			req.Year, req.Month, req.Category, req.Channel).First(&budget).Error

		if err == nil {
			// Update existing
			budget.BudgetAmount = req.BudgetAmount
			if err := tx.Save(&budget).Error; err != nil {
				return err
			}
		} else if err == gorm.ErrRecordNotFound {
			// Create new
			budget = models.MonthBudget{
				Year:         req.Year,
				Month:        req.Month,
				Category:     req.Category,
				Channel:      req.Channel,
				BudgetAmount: req.BudgetAmount,
			}
			if err := tx.Create(&budget).Error; err != nil {
				return err
			}
		} else {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(budget)
}
