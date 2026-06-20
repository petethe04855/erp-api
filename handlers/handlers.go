package handlers

import (
	"fmt"
	"strconv"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Helper: Get audit trail standard time string
func getNowStr() string {
	return time.Now().Format("2006-01-02T15:04")
}

// NextID generates next serial ID like prefix0001
func NextID(db *gorm.DB, prefix string, model interface{}, idField string) (string, error) {
	var lastID string
	err := db.Model(model).Order(fmt.Sprintf("%s DESC", idField)).Limit(1).Pluck(idField, &lastID).Error
	if err != nil {
		return "", err
	}

	suffix := 1
	if lastID != "" {
		if len(lastID) > len(prefix) {
			numPart := lastID[len(prefix):]
			if val, err := strconv.Atoi(numPart); err == nil {
				suffix = val + 1
			}
		}
	}

	return fmt.Sprintf("%s%04d", prefix, suffix), nil
}

// GET /api/init
// Returns the entire state of the ERP database
func InitState(c *fiber.Ctx) error {
	var quotations []models.Quotation
	var salesOrders []models.SalesOrder
	var invoices []models.Invoice
	var purchaseRequests []models.PurchaseRequest
	var purchaseOrders []models.PurchaseOrder
	var goodsReceives []models.GoodsReceive
	var stockMovements []models.StockMovement
	var products []models.Product
	var stockLots []models.StockLot
	var samplingCampaigns []models.SamplingCampaign
	var goodsIssues []models.GoodsIssue
	var stockReturns []models.StockReturn
	var stockAdjustments []models.StockAdjustment
	var stockTransfers []models.StockTransfer
	var expenses []models.Expense
	var budgets []models.MonthBudget
	var bundleComponents []models.BundleComponent
	var tiktokOrders []models.TiktokOrder
	var liveSessions []models.LiveSession
	var contentSchedule []models.ContentScheduleItem
	var manualOrders []models.ManualOrder
	var users []models.AppUser

	var company models.CompanySettings
	var notifications models.NotificationSettings
	var modules models.ModuleSettings
	var livePayroll models.LivePayrollSettings

	// Preloads for relationships
	database.DB.Preload("Lines").Find(&quotations)
	database.DB.Preload("Lines").Preload("AuditTrail").Find(&salesOrders)
	database.DB.Preload("AuditTrail").Find(&invoices)
	database.DB.Preload("Items").Find(&purchaseRequests)
	database.DB.Preload("Items").Preload("AuditTrail").Find(&purchaseOrders)
	database.DB.Preload("Items").Preload("AuditTrail").Find(&goodsReceives)
	database.DB.Find(&stockMovements)
	database.DB.Find(&products)
	database.DB.Find(&stockLots)
	database.DB.Preload("Recipients").Find(&samplingCampaigns)
	database.DB.Find(&goodsIssues)
	database.DB.Find(&stockReturns)
	database.DB.Preload("Items").Find(&stockAdjustments)
	database.DB.Find(&stockTransfers)
	database.DB.Find(&expenses)
	database.DB.Find(&budgets)
	database.DB.Find(&bundleComponents)
	database.DB.Find(&tiktokOrders)
	database.DB.Find(&liveSessions)
	database.DB.Find(&contentSchedule)
	database.DB.Find(&manualOrders)
	database.DB.Find(&users)

	// Fetch settings or create defaults
	if err := database.DB.First(&company).Error; err != nil {
		company = models.CompanySettings{
			Name: "Chawy Pet Food", TaxID: "0123456789012", Address: "123 ถ.สุขุมวิท แขวงคลองเตย เขตคลองเตย กรุงเทพฯ 10110", Phone: "02-123-4567", Email: "hello@chawypet.com", Website: "www.chawypet.com", Currency: "THB", VatRate: 7, InvoicePrefix: "INV-2026-", SoPrefix: "SO-2026-",
		}
		database.DB.Create(&company)
	}
	if err := database.DB.First(&notifications).Error; err != nil {
		notifications = models.NotificationSettings{
			NearExpiry: true, NearExpiryDays: 30, LowStock: true, LatePO: true, NewSO: true, PaymentDue: true,
		}
		database.DB.Create(&notifications)
	}
	if err := database.DB.First(&modules).Error; err != nil {
		modules = models.ModuleSettings{
			Quotation: true, SalesOrders: true, Invoice: true, Returns: true, PurchaseReq: true, PurchaseOrder: true, SkuMaster: true, StockBalance: true, GoodsReceive: true, GoodsIssue: true, StockTransfer: true, StockCheck: true, Expenses: true, PlReport: true, Budget: true, TiktokOrders: true, LiveContent: true, ManualOrder: true, TiktokCalculator: true, Sampling: true, UserManagement: true, TiktokSetup: true,
		}
		database.DB.Create(&modules)
	}
	if err := database.DB.First(&livePayroll).Error; err != nil {
		livePayroll = models.LivePayrollSettings{
			HourlyRate: 120, ClipBonus: 100,
		}
		database.DB.Create(&livePayroll)
	}

	return c.JSON(fiber.Map{
		"quotations":        quotations,
		"salesOrders":       salesOrders,
		"invoices":          invoices,
		"purchaseRequests":  purchaseRequests,
		"purchaseOrders":    purchaseOrders,
		"goodsReceives":     goodsReceives,
		"stockMovements":    stockMovements,
		"products":          products,
		"stockLots":         stockLots,
		"samplingCampaigns": samplingCampaigns,
		"goodsIssues":       goodsIssues,
		"stockReturns":      stockReturns,
		"stockAdjustments":  stockAdjustments,
		"stockTransfers":    stockTransfers,
		"expenses":          expenses,
		"budgets":           budgets,
		"bundleComponents":  bundleComponents,
		"tiktokOrders":      tiktokOrders,
		"liveSessions":      liveSessions,
		"contentSchedule":   contentSchedule,
		"manualOrders":      manualOrders,
		"users":             users,
		"settings": fiber.Map{
			"company":       company,
			"notifications": notifications,
			"modules":       modules,
			"livePayroll":   livePayroll,
		},
	})
}
