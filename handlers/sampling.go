package handlers

import (
	"fmt"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// POST /api/sampling-campaigns
func CreateSamplingCampaign(c *fiber.Ctx) error {
	var campaign models.SamplingCampaign
	if err := c.BodyParser(&campaign); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		id, err := NextID(tx, "SAMP-", &models.SamplingCampaign{}, "id")
		if err != nil {
			return err
		}
		campaign.ID = id
		campaign.GivenQty = 0
		campaign.Status = "Active"

		if err := tx.Create(&campaign).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Recipients").First(&campaign, "id = ?", campaign.ID)
	return c.JSON(campaign)
}

// POST /api/sampling-campaigns/:id/recipients
func AddSamplingRecipient(c *fiber.Ctx) error {
	campaignID := c.Params("id")
	var recipient models.SamplingRecipient
	if err := c.BodyParser(&recipient); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var campaign models.SamplingCampaign
	if err := database.DB.Preload("Recipients").First(&campaign, "id = ?", campaignID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sampling campaign not found"})
	}

	if campaign.Status != "Active" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Campaign is not Active"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		recipient.ID = fmt.Sprintf("RCP-%d", time.Now().UnixNano())
		recipient.CampaignID = campaignID
		if recipient.Date == "" {
			recipient.Date = time.Now().Format("2006-01-02")
		}

		if err := tx.Create(&recipient).Error; err != nil {
			return err
		}

		// Update campaign quantities
		campaign.GivenQty += recipient.QtyGiven
		if err := tx.Save(&campaign).Error; err != nil {
			return err
		}

		// Deduct stock of the product associated with campaign
		var product models.Product
		if err := tx.First(&product, "sku = ?", campaign.SKU).Error; err == nil {
			product.Stock -= recipient.QtyGiven
			if product.Stock < 0 {
				product.Stock = 0
			}
			if err := tx.Save(&product).Error; err != nil {
				return err
			}

			// Create OUT Stock Movement
			movement := models.StockMovement{
				ID:        fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), campaign.SKU),
				SKU:       campaign.SKU,
				Type:      "OUT",
				Qty:       recipient.QtyGiven,
				RefDoc:    campaignID,
				Date:      recipient.Date,
				Note:      fmt.Sprintf("ฟรีแซมพลิง: %s ให้กับ %s", campaign.Name, recipient.Name),
				ChangedBy: username.(string),
			}
			if err := tx.Create(&movement).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Recipients").First(&campaign, "id = ?", campaignID)
	return c.JSON(campaign)
}

// PUT /api/sampling-campaigns/:id/status
func UpdateSamplingStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var campaign models.SamplingCampaign
	if err := database.DB.First(&campaign, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sampling campaign not found"})
	}

	campaign.Status = req.Status
	if err := database.DB.Save(&campaign).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Recipients").First(&campaign, "id = ?", id)
	return c.JSON(campaign)
}
