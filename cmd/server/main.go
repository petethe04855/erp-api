package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"

	"chawy-erp-api/database"
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"
	"chawy-erp-api/models"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables from system")
	}

	// Connect to database
	database.ConnectDB()
	if os.Getenv("CLEAN_DB") == "true" {
		database.CleanMockData()
	}
	database.SeedData()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := fiber.New(fiber.Config{
		AppName: "Chawy ERP API v2",
	})

	// CORS configuration
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))
	app.Use(middleware.StandardizeJSONResponse)

	// Health Check Route
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// Auth Routes
	app.Post("/api/auth/login", handlers.Login)
	app.Get("/api/auth/me", middleware.AuthRequired, handlers.GetCurrentUser)

	// Protected API Routes Group
	api := app.Group("/api", middleware.AuthRequired)

	// Users
	api.Get("/users", middleware.RequireRoles("owner"), handlers.ListResource(func() interface{} { return &[]models.AppUser{} }))
	api.Get("/users/:id", middleware.RequireRoles("owner"), handlers.GetResource(func() interface{} { return &models.AppUser{} }, "id", "id"))
	api.Post("/users", middleware.RequireRoles("owner"), handlers.CreateUser)
	api.Put("/users/:id", middleware.RequireRoles("owner"), handlers.UpdateUser)
	api.Put("/users/:id/status", middleware.RequireRoles("owner"), handlers.UpdateUserStatus)

	// Exports
	api.Get("/export/sales-orders", handlers.ExportSalesOrders)
	api.Get("/export/invoices", handlers.ExportInvoices)
	api.Get("/export/returns", handlers.ExportReturns)
	api.Get("/export/purchase-orders", handlers.ExportPurchaseOrders)
	api.Get("/export/expenses", handlers.ExportExpenses)
	api.Get("/export/pl", handlers.ExportPL)
	api.Get("/export/budget", handlers.ExportBudget)
	api.Get("/export/tiktok-orders", handlers.ExportTiktokOrders)

	// Settings
	api.Get("/settings", handlers.GetSettings)
	api.Put("/settings", middleware.RequireRoles("owner"), handlers.UpdateSettings)

	// Products
	api.Get("/products", handlers.ListResource(func() interface{} { return &[]models.Product{} }))
	api.Get("/products/:sku", handlers.GetResource(func() interface{} { return &models.Product{} }, "sku", "sku"))
	api.Post("/products", handlers.CreateProduct)
	api.Put("/products/:sku", handlers.UpdateProduct)
	api.Delete("/products/:sku", middleware.RequireRoles("owner", "warehouse"), handlers.DeleteProduct)
	api.Post("/bundle-components", handlers.SetBundleComponents)
	api.Get("/bundle-components", handlers.ListResource(func() interface{} { return &[]models.BundleComponent{} }))
	api.Get("/bundle-components/:sku", handlers.ListResourceWhere(func() interface{} { return &[]models.BundleComponent{} }, "bundle_sku", "sku"))

	// Quotations
	api.Get("/quotations", handlers.ListResource(func() interface{} { return &[]models.Quotation{} }, "Lines"))
	api.Get("/quotations/:id", handlers.GetResource(func() interface{} { return &models.Quotation{} }, "id", "id", "Lines"))
	api.Post("/quotations", handlers.CreateQuotation)
	api.Put("/quotations/:id/status", handlers.UpdateQuotationStatus)
	api.Post("/quotations/:id/convert", handlers.ConvertQuotationToSalesOrder)

	// Sales Orders
	api.Get("/sales-orders", handlers.ListResource(func() interface{} { return &[]models.SalesOrder{} }, "Lines", "AuditTrail"))
	api.Get("/sales-orders/:id", handlers.GetResource(func() interface{} { return &models.SalesOrder{} }, "id", "id", "Lines", "AuditTrail"))
	api.Post("/sales-orders", handlers.CreateSalesOrder)
	api.Put("/sales-orders/:id/status", handlers.UpdateSalesOrderStatus)

	// Invoices
	api.Get("/invoices", handlers.ListResource(func() interface{} { return &[]models.Invoice{} }, "AuditTrail"))
	api.Get("/invoices/:id", handlers.GetResource(func() interface{} { return &models.Invoice{} }, "id", "id", "AuditTrail"))
	api.Post("/invoices", handlers.CreateInvoice)
	api.Post("/invoices/from-so/:soId", handlers.CreateInvoiceFromSO)
	api.Post("/invoices/:id/payment", handlers.RecordPayment)

	// Purchase Requests
	api.Get("/purchase-requests", handlers.ListResource(func() interface{} { return &[]models.PurchaseRequest{} }, "Items"))
	api.Get("/purchase-requests/:id", handlers.GetResource(func() interface{} { return &models.PurchaseRequest{} }, "id", "id", "Items"))
	api.Post("/purchase-requests", handlers.CreatePurchaseRequest)
	api.Put("/purchase-requests/:id/status", handlers.UpdatePRStatus)
	api.Post("/purchase-requests/:id/convert", handlers.ConvertPRtoPO)

	// Purchase Orders
	api.Get("/purchase-orders", handlers.ListResource(func() interface{} { return &[]models.PurchaseOrder{} }, "Items", "AuditTrail"))
	api.Get("/purchase-orders/:id", handlers.GetResource(func() interface{} { return &models.PurchaseOrder{} }, "id", "id", "Items", "AuditTrail"))
	api.Post("/purchase-orders", handlers.CreatePurchaseOrder)
	api.Put("/purchase-orders/:id/status", handlers.UpdatePOStatus)

	// Goods Receive
	api.Get("/goods-receives", handlers.ListResource(func() interface{} { return &[]models.GoodsReceive{} }, "Items", "AuditTrail"))
	api.Get("/goods-receives/:id", handlers.GetResource(func() interface{} { return &models.GoodsReceive{} }, "id", "id", "Items", "AuditTrail"))
	api.Post("/goods-receives", handlers.CreateGoodsReceive)

	// Sampling
	api.Get("/sampling-campaigns", handlers.ListResource(func() interface{} { return &[]models.SamplingCampaign{} }, "Recipients"))
	api.Get("/sampling-campaigns/:id", handlers.GetResource(func() interface{} { return &models.SamplingCampaign{} }, "id", "id", "Recipients"))
	api.Post("/sampling-campaigns", handlers.CreateSamplingCampaign)
	api.Post("/sampling-campaigns/:id/recipients", handlers.AddSamplingRecipient)
	api.Put("/sampling-campaigns/:id/status", handlers.UpdateSamplingStatus)

	// Stock Operations
	api.Get("/goods-issues", handlers.ListResource(func() interface{} { return &[]models.GoodsIssue{} }))
	api.Get("/goods-issues/:id", handlers.GetResource(func() interface{} { return &models.GoodsIssue{} }, "id", "id"))
	api.Post("/goods-issues", handlers.CreateGoodsIssue)
	api.Get("/stock-returns", handlers.ListResource(func() interface{} { return &[]models.StockReturn{} }))
	api.Get("/stock-returns/:id", handlers.GetResource(func() interface{} { return &models.StockReturn{} }, "id", "id"))
	api.Post("/stock-returns", handlers.CreateStockReturn)
	api.Put("/stock-returns/:id/status", handlers.UpdateStockReturnStatus)
	api.Get("/stock-adjustments", handlers.ListResource(func() interface{} { return &[]models.StockAdjustment{} }, "Items"))
	api.Get("/stock-adjustments/:id", handlers.GetResource(func() interface{} { return &models.StockAdjustment{} }, "id", "id", "Items"))
	api.Post("/stock-adjustments", handlers.CreateStockAdjustment)
	api.Get("/stock-transfers", handlers.ListResource(func() interface{} { return &[]models.StockTransfer{} }))
	api.Get("/stock-transfers/:id", handlers.GetResource(func() interface{} { return &models.StockTransfer{} }, "id", "id"))
	api.Post("/stock-transfers", handlers.CreateStockTransfer)
	api.Get("/stock-lots", handlers.ListResource(func() interface{} { return &[]models.StockLot{} }))
	api.Get("/stock-lots/:id", handlers.GetResource(func() interface{} { return &models.StockLot{} }, "id", "id"))
	api.Get("/stock-movements", handlers.ListResource(func() interface{} { return &[]models.StockMovement{} }))
	api.Get("/stock-movements/:id", handlers.GetResource(func() interface{} { return &models.StockMovement{} }, "id", "id"))

	// Finance
	api.Get("/expenses", handlers.ListResource(func() interface{} { return &[]models.Expense{} }))
	api.Get("/expenses/:id", handlers.GetResource(func() interface{} { return &models.Expense{} }, "id", "id"))
	api.Post("/expenses", handlers.CreateExpense)
	api.Put("/expenses/:id", handlers.UpdateExpense)
	api.Delete("/expenses/:id", middleware.RequireRoles("owner", "accountant"), handlers.DeleteExpense)
	api.Get("/budgets", handlers.ListResource(func() interface{} { return &[]models.MonthBudget{} }))
	api.Get("/budgets/:id", handlers.GetResource(func() interface{} { return &models.MonthBudget{} }, "id", "id"))
	api.Post("/budgets", handlers.UpsertBudget)
	api.Put("/budgets/:id", handlers.UpdateBudget)
	api.Delete("/budgets/:id", middleware.RequireRoles("owner", "accountant"), handlers.DeleteBudget)

	// Platform & Channels
	api.Get("/tiktok-orders", handlers.ListResource(func() interface{} { return &[]models.TiktokOrder{} }))
	api.Get("/tiktok-orders/:id", handlers.GetResource(func() interface{} { return &models.TiktokOrder{} }, "id", "id"))
	api.Post("/tiktok-orders", handlers.CreateTiktokOrder)
	api.Put("/tiktok-orders/:id/imported", handlers.MarkTiktokOrderImported)
	api.Post("/tiktok-orders/:id/settle", handlers.ApplyTiktokSettlement)
	api.Get("/live-sessions", handlers.ListResource(func() interface{} { return &[]models.LiveSession{} }))
	api.Get("/live-sessions/:id", handlers.GetResource(func() interface{} { return &models.LiveSession{} }, "id", "id"))
	api.Post("/live-sessions", handlers.CreateLiveSession)
	api.Put("/live-sessions/:id/status", handlers.UpdateLiveSessionStatus)
	api.Get("/content-schedule", handlers.ListResource(func() interface{} { return &[]models.ContentScheduleItem{} }))
	api.Get("/content-schedule/:id", handlers.GetResource(func() interface{} { return &models.ContentScheduleItem{} }, "id", "id"))
	api.Post("/content-schedule", handlers.CreateContentSchedule)
	api.Put("/content-schedule/:id", handlers.UpdateContentSchedule)
	api.Put("/content-schedule/:id/status", handlers.UpdateContentScheduleStatus)
	api.Delete("/content-schedule/:id", handlers.DeleteContentSchedule)
	api.Get("/manual-orders", handlers.ListResource(func() interface{} { return &[]models.ManualOrder{} }))
	api.Get("/manual-orders/:id", handlers.GetResource(func() interface{} { return &models.ManualOrder{} }, "id", "id"))
	api.Post("/manual-orders", handlers.CreateManualOrder)
	api.Put("/manual-orders/:id", handlers.UpdateManualOrder)
	api.Delete("/manual-orders/:id", handlers.DeleteManualOrder)

	log.Printf("Starting server on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
