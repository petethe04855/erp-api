package handlers

import (
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// POST /api/tiktok-orders
func CreateTiktokOrder(c *fiber.Ctx) error {
	var order models.TiktokOrder
	if err := c.BodyParser(&order); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if order.ID == "" {
			id, err := NextID(tx, "TT-", &models.TiktokOrder{}, "id")
			if err != nil {
				return err
			}
			order.ID = id
		}
		order.Date = time.Now().Format("2006-01-02")
		order.Imported = false

		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(order)
}

// PUT /api/tiktok-orders/:id/imported
func MarkTiktokOrderImported(c *fiber.Ctx) error {
	id := c.Params("id")
	var order models.TiktokOrder
	if err := database.DB.First(&order, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tiktok order not found"})
	}

	order.Imported = true
	if err := database.DB.Save(&order).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(order)
}

// POST /api/tiktok-orders/:id/settle
func ApplyTiktokSettlement(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		NetRevenue    float64 `json:"netRevenue"`
		PlatformFee   float64 `json:"platformFee"`
		SettlementRef string  `json:"settlementRef"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var order models.TiktokOrder
	if err := database.DB.First(&order, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Tiktok order not found"})
	}

	order.NetRevenue = req.NetRevenue
	order.PlatformFee = req.PlatformFee
	order.SettlementRef = req.SettlementRef
	order.Settled = true

	if err := database.DB.Save(&order).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(order)
}

// POST /api/live-sessions
func CreateLiveSession(c *fiber.Ctx) error {
	var session models.LiveSession
	if err := c.BodyParser(&session); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		id, err := NextID(tx, "LS-2026-", &models.LiveSession{}, "id")
		if err != nil {
			return err
		}
		session.ID = id
		session.Status = "Pending Approval"
		session.CreatedBy = username.(string)
		session.UpdatedBy = username.(string)
		session.UpdatedAt = time.Now().Format("2006-01-02T15:04")

		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(session)
}

// PUT /api/live-sessions/:id/status
func UpdateLiveSessionStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status     string `json:"status"`
		ApprovedBy string `json:"approvedBy"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var session models.LiveSession
	if err := database.DB.First(&session, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Live session not found"})
	}

	session.Status = req.Status
	session.ApprovedBy = req.ApprovedBy
	session.UpdatedAt = time.Now().Format("2006-01-02T15:04")

	if err := database.DB.Save(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(session)
}

// POST /api/content-schedule
func CreateContentSchedule(c *fiber.Ctx) error {
	var item models.ContentScheduleItem
	if err := c.BodyParser(&item); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		id, err := NextID(tx, "CS-", &models.ContentScheduleItem{}, "id")
		if err != nil {
			return err
		}
		item.ID = id
		item.CreatedAt = time.Now().Format("2006-01-02T15:04")

		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(item)
}

// PUT /api/content-schedule/:id/status
func UpdateContentScheduleStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var item models.ContentScheduleItem
	if err := database.DB.First(&item, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Content schedule item not found"})
	}

	item.Status = req.Status
	if err := database.DB.Save(&item).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(item)
}

// POST /api/manual-orders
func CreateManualOrder(c *fiber.Ctx) error {
	var order models.ManualOrder
	if err := c.BodyParser(&order); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		id, err := NextID(tx, "MO-2026-", &models.ManualOrder{}, "id")
		if err != nil {
			return err
		}
		order.ID = id
		if order.Date == "" {
			order.Date = time.Now().Format("2006-01-02")
		}

		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(order)
}
