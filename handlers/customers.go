package handlers

import (
	"fmt"
	"strings"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// POST /api/customers
func CreateCustomer(c *fiber.Ctx) error {
	var cust models.Customer
	if err := c.BodyParser(&cust); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	cust.Name = strings.TrimSpace(cust.Name)
	cust.Address = strings.TrimSpace(cust.Address)
	cust.TaxID = strings.TrimSpace(cust.TaxID)

	if cust.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Customer name is required"})
	}

	now := time.Now().Format("2006-01-02T15:04")
	cust.CreatedAt = now
	cust.UpdatedAt = now

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if cust.ID == "" {
			code, err := NextCode(tx, "CUST-", &models.Customer{}, "id")
			if err != nil {
				return err
			}
			cust.ID = code
		}
		return tx.Create(&cust).Error
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(cust)
}

// PUT /api/customers/:id
func UpdateCustomer(c *fiber.Ctx) error {
	id := c.Params("id")
	var existing models.Customer
	if err := database.DB.First(&existing, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if val, ok := body["name"]; ok && val != nil {
		existing.Name = strings.TrimSpace(fmt.Sprint(val))
	}
	if val, ok := body["taxId"]; ok {
		if val == nil { existing.TaxID = "" } else { existing.TaxID = strings.TrimSpace(fmt.Sprint(val)) }
	}
	if val, ok := body["branch"]; ok {
		if val == nil { existing.Branch = "" } else { existing.Branch = strings.TrimSpace(fmt.Sprint(val)) }
	}
	if val, ok := body["phone"]; ok {
		if val == nil { existing.Phone = "" } else { existing.Phone = strings.TrimSpace(fmt.Sprint(val)) }
	}
	if val, ok := body["email"]; ok {
		if val == nil { existing.Email = "" } else { existing.Email = strings.TrimSpace(fmt.Sprint(val)) }
	}
	if val, ok := body["website"]; ok {
		if val == nil { existing.Website = "" } else { existing.Website = strings.TrimSpace(fmt.Sprint(val)) }
	}
	if val, ok := body["contactPerson"]; ok {
		if val == nil { existing.ContactPerson = "" } else { existing.ContactPerson = strings.TrimSpace(fmt.Sprint(val)) }
	}
	if val, ok := body["address"]; ok && val != nil {
		existing.Address = strings.TrimSpace(fmt.Sprint(val))
	}
	if val, ok := body["logoUrl"]; ok {
		if val == nil { existing.LogoURL = "" } else { existing.LogoURL = strings.TrimSpace(fmt.Sprint(val)) }
	}
	existing.UpdatedAt = time.Now().Format("2006-01-02T15:04")

	if err := database.DB.Save(&existing).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(existing)
}

// DELETE /api/customers/:id
func DeleteCustomer(c *fiber.Ctx) error {
	id := c.Params("id")
	return deleteSimple(c, &models.Customer{}, "id", id)
}
