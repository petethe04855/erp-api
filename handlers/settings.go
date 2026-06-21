package handlers

import (
	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UpdateSettingsRequest struct {
	Company       *models.CompanySettings      `json:"company"`
	Notifications *models.NotificationSettings `json:"notifications"`
	Modules       *models.ModuleSettings       `json:"modules"`
	LivePayroll   *models.LivePayrollSettings  `json:"livePayroll"`
}

// GET /api/settings
func GetSettings(c *fiber.Ctx) error {
	var company models.CompanySettings
	var notifications models.NotificationSettings
	var modules models.ModuleSettings
	var livePayroll models.LivePayrollSettings

	if err := database.DB.First(&company).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := database.DB.First(&notifications).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := database.DB.First(&modules).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := database.DB.First(&livePayroll).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"company":       company,
		"notifications": notifications,
		"modules":       modules,
		"livePayroll":   livePayroll,
	})
}

// PUT /api/settings
func UpdateSettings(c *fiber.Ctx) error {
	var req UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if req.Company != nil {
			var comp models.CompanySettings
			tx.First(&comp)
			// Map values
			comp.Name = req.Company.Name
			comp.TaxID = req.Company.TaxID
			comp.Address = req.Company.Address
			comp.Phone = req.Company.Phone
			comp.Email = req.Company.Email
			comp.Website = req.Company.Website
			comp.Currency = req.Company.Currency
			comp.VatRate = req.Company.VatRate
			comp.InvoicePrefix = req.Company.InvoicePrefix
			comp.SoPrefix = req.Company.SoPrefix
			if err := tx.Save(&comp).Error; err != nil {
				return err
			}
		}

		if req.Notifications != nil {
			var notif models.NotificationSettings
			tx.First(&notif)
			notif.NearExpiry = req.Notifications.NearExpiry
			notif.NearExpiryDays = req.Notifications.NearExpiryDays
			notif.LowStock = req.Notifications.LowStock
			notif.LatePO = req.Notifications.LatePO
			notif.NewSO = req.Notifications.NewSO
			notif.PaymentDue = req.Notifications.PaymentDue
			if err := tx.Save(&notif).Error; err != nil {
				return err
			}
		}

		if req.Modules != nil {
			var mod models.ModuleSettings
			tx.First(&mod)
			mod.Quotation = req.Modules.Quotation
			mod.SalesOrders = req.Modules.SalesOrders
			mod.Invoice = req.Modules.Invoice
			mod.Returns = req.Modules.Returns
			mod.PurchaseReq = req.Modules.PurchaseReq
			mod.PurchaseOrder = req.Modules.PurchaseOrder
			mod.SkuMaster = req.Modules.SkuMaster
			mod.StockBalance = req.Modules.StockBalance
			mod.GoodsReceive = req.Modules.GoodsReceive
			mod.GoodsIssue = req.Modules.GoodsIssue
			mod.StockTransfer = req.Modules.StockTransfer
			mod.StockCheck = req.Modules.StockCheck
			mod.Expenses = req.Modules.Expenses
			mod.PlReport = req.Modules.PlReport
			mod.Budget = req.Modules.Budget
			mod.TiktokOrders = req.Modules.TiktokOrders
			mod.LiveContent = req.Modules.LiveContent
			mod.ManualOrder = req.Modules.ManualOrder
			mod.TiktokCalculator = req.Modules.TiktokCalculator
			mod.Sampling = req.Modules.Sampling
			mod.UserManagement = req.Modules.UserManagement
			mod.TiktokSetup = req.Modules.TiktokSetup
			if err := tx.Save(&mod).Error; err != nil {
				return err
			}
		}

		if req.LivePayroll != nil {
			var payroll models.LivePayrollSettings
			tx.First(&payroll)
			payroll.HourlyRate = req.LivePayroll.HourlyRate
			payroll.ClipBonus = req.LivePayroll.ClipBonus
			if err := tx.Save(&payroll).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true})
}
