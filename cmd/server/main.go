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

	// Init State
	api.Get("/init", handlers.InitState)

	// Users
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
	api.Put("/settings", middleware.RequireRoles("owner"), handlers.UpdateSettings)

	// Products
	api.Post("/products", handlers.CreateProduct)
	api.Put("/products/:sku", handlers.UpdateProduct)
	api.Delete("/products/:sku", middleware.RequireRoles("owner", "warehouse"), handlers.DeleteProduct)
	api.Post("/bundle-components", handlers.SetBundleComponents)

	// Quotations
	api.Post("/quotations", handlers.CreateQuotation)
	api.Put("/quotations/:id/status", handlers.UpdateQuotationStatus)
	api.Post("/quotations/:id/convert", handlers.ConvertQuotationToSalesOrder)

	// Sales Orders
	api.Post("/sales-orders", handlers.CreateSalesOrder)
	api.Put("/sales-orders/:id/status", handlers.UpdateSalesOrderStatus)

	// Invoices
	api.Post("/invoices", handlers.CreateInvoice)
	api.Post("/invoices/from-so/:soId", handlers.CreateInvoiceFromSO)
	api.Post("/invoices/:id/payment", handlers.RecordPayment)

	// Purchase Requests
	api.Post("/purchase-requests", handlers.CreatePurchaseRequest)
	api.Put("/purchase-requests/:id/status", handlers.UpdatePRStatus)
	api.Post("/purchase-requests/:id/convert", handlers.ConvertPRtoPO)

	// Purchase Orders
	api.Post("/purchase-orders", handlers.CreatePurchaseOrder)
	api.Put("/purchase-orders/:id/status", handlers.UpdatePOStatus)

	// Goods Receive
	api.Post("/goods-receives", handlers.CreateGoodsReceive)

	// Sampling
	api.Post("/sampling-campaigns", handlers.CreateSamplingCampaign)
	api.Post("/sampling-campaigns/:id/recipients", handlers.AddSamplingRecipient)
	api.Put("/sampling-campaigns/:id/status", handlers.UpdateSamplingStatus)

	// Stock Operations
	api.Post("/goods-issues", handlers.CreateGoodsIssue)
	api.Post("/stock-returns", handlers.CreateStockReturn)
	api.Put("/stock-returns/:id/status", handlers.UpdateStockReturnStatus)
	api.Post("/stock-adjustments", handlers.CreateStockAdjustment)
	api.Post("/stock-transfers", handlers.CreateStockTransfer)

	// Finance
	api.Post("/expenses", handlers.CreateExpense)
	api.Post("/budgets", handlers.UpsertBudget)

	// Platform & Channels
	api.Post("/tiktok-orders", handlers.CreateTiktokOrder)
	api.Put("/tiktok-orders/:id/imported", handlers.MarkTiktokOrderImported)
	api.Post("/tiktok-orders/:id/settle", handlers.ApplyTiktokSettlement)
	api.Post("/live-sessions", handlers.CreateLiveSession)
	api.Put("/live-sessions/:id/status", handlers.UpdateLiveSessionStatus)
	api.Post("/content-schedule", handlers.CreateContentSchedule)
	api.Put("/content-schedule/:id/status", handlers.UpdateContentScheduleStatus)
	api.Post("/manual-orders", handlers.CreateManualOrder)

	log.Printf("Starting server on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
