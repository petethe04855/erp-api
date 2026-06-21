package handlers

import (
	"chawy-erp-api/database"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type resourceFactory func() interface{}

// ListResource builds a standard GET collection handler.
func ListResource(factory resourceFactory, preloads ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		value := factory()
		query := database.DB
		for _, preload := range preloads {
			query = query.Preload(preload)
		}
		if err := query.Find(value).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(value)
	}
}

// ListResourceWhere builds a filtered GET collection handler.
func ListResourceWhere(factory resourceFactory, column, param string, preloads ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		value := factory()
		query := database.DB.Where(column+" = ?", c.Params(param))
		for _, preload := range preloads {
			query = query.Preload(preload)
		}
		if err := query.Find(value).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(value)
	}
}

// GetResource builds a standard GET detail handler using a route parameter.
func GetResource(factory resourceFactory, column, param string, preloads ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		value := factory()
		query := database.DB
		for _, preload := range preloads {
			query = query.Preload(preload)
		}
		if err := query.First(value, column+" = ?", c.Params(param)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Resource not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(value)
	}
}
