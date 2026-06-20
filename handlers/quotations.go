package handlers

import (
	"fmt"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// POST /api/quotations
func CreateQuotation(c *fiber.Ctx) error {
	var qt models.Quotation
	if err := c.BodyParser(&qt); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Generate custom ID
		id, err := NextID(tx, "QT-2026-", &models.Quotation{}, "id")
		if err != nil {
			return err
		}
		qt.ID = id

		// Recalculate amount if lines exist
		var totalAmount float64
		for i := range qt.Lines {
			qt.Lines[i].QuotationID = id
			totalAmount += float64(qt.Lines[i].Qty) * qt.Lines[i].Price
		}
		qt.Amount = totalAmount

		if err := tx.Create(&qt).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Lines").First(&qt, "id = ?", qt.ID)
	return c.JSON(qt)
}

// PUT /api/quotations/:id/status
func UpdateQuotationStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var qt models.Quotation
	if err := database.DB.First(&qt, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Quotation not found"})
	}

	qt.Status = req.Status
	if err := database.DB.Save(&qt).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Lines").First(&qt, "id = ?", id)
	return c.JSON(qt)
}

// POST /api/quotations/:id/convert
func ConvertQuotationToSalesOrder(c *fiber.Ctx) error {
	id := c.Params("id")
	var qt models.Quotation
	if err := database.DB.Preload("Lines").First(&qt, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Quotation not found"})
	}

	var so models.SalesOrder
	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Update quotation status
		qt.Status = "Converted"
		if err := tx.Save(&qt).Error; err != nil {
			return err
		}

		// Generate Next SO ID
		soID, err := NextID(tx, "SO-2026-", &models.SalesOrder{}, "id")
		if err != nil {
			return err
		}

		totalItems := 0
		var soLines []models.SalesOrderLine
		for _, ql := range qt.Lines {
			soLines = append(soLines, models.SalesOrderLine{
				OrderID: soID,
				SKU:     ql.SKU,
				Qty:     ql.Qty,
			})
			totalItems += ql.Qty

			// Update ReservedQty on products
			var prod models.Product
			if err := tx.First(&prod, "sku = ?", ql.SKU).Error; err == nil {
				prod.ReservedQty += ql.Qty
				if err := tx.Save(&prod).Error; err != nil {
					return err
				}
			}
		}

		so = models.SalesOrder{
			ID:        soID,
			Customer:  qt.Customer,
			Date:      time.Now().Format("2006-01-02"),
			Amount:    qt.Amount,
			Status:    "Pending Payment",
			Channel:   "Manual",
			Items:     totalItems,
			Lines:     soLines,
			QtRef:     qt.ID,
			SourceRef: "",
		}

		if err := tx.Create(&so).Error; err != nil {
			return err
		}

		// Add audit trail event
		audit := models.AuditEvent{
			OwnerID:   so.ID,
			OwnerType: "sales_orders",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("Converted from Quotation %s", qt.ID),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Lines").Preload("AuditTrail").First(&so, "id = ?", so.ID)
	return c.JSON(so)
}
