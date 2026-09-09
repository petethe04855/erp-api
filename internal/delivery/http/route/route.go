package route

import (
	"chawy-erp-api/internal/delivery/http/handler"
	"chawy-erp-api/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	App               *fiber.App
	AuthHandler       *handler.AuthHandler
	SKUHandler        *handler.SKUHandler
	StockHandler      *handler.StockHandler
	BundleHandler     *handler.BundleHandler
	CustomerHandler   *handler.CustomerHandler
	OrderHandler      *handler.OrderHandler
	PurchasingHandler *handler.PurchasingHandler
	InvoiceHandler    *handler.InvoiceHandler
	ReportHandler     *handler.ReportHandler
	WorkspaceHandler  *handler.WorkspaceHandler
	TikTokHandler     *handler.TikTokHandler
	SettingsHandler   *handler.SettingsHandler
	JWTSecret         string
}

func RegisterRoutes(cfg Config) {
	api := cfg.App.Group("/api/v1")

	// Public Health Check
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "v2",
			"message": "Chawy ERP Clean Architecture API v2",
		})
	})

	// Auth Routes (Public)
	authGroup := api.Group("/auth")
	authGroup.Post("/register", cfg.AuthHandler.Register)
	authGroup.Post("/login", cfg.AuthHandler.Login)
	authGroup.Get("/me", middleware.AuthMiddleware(cfg.JWTSecret), cfg.AuthHandler.Me)

	// Protected Routes Group
	protected := api.Group("/", middleware.AuthMiddleware(cfg.JWTSecret))

	// RBAC Matrix (API-01)
	protected.Get("/rbac/permission-matrix", cfg.AuthHandler.GetPermissionMatrix)

	// User Management Routes (API-01, API-31) - Restricted to 'owner' and 'admin'
	users := protected.Group("/users", middleware.RequireRole("owner", "admin"))
	users.Get("/", cfg.AuthHandler.ListUsers)
	users.Get("/:id", cfg.AuthHandler.GetUserByID)
	users.Post("/", cfg.AuthHandler.CreateUser)
	users.Put("/:id", cfg.AuthHandler.UpdateUser)
	users.Put("/:id/status", cfg.AuthHandler.UpdateUserStatus)
	users.Delete("/:id", cfg.AuthHandler.DeleteUser)

	// SKU Routes
	skus := protected.Group("/skus")
	skus.Get("/", cfg.SKUHandler.List)
	skus.Post("/", middleware.RequireRole("owner", "admin", "warehouse", "sales"), cfg.SKUHandler.Create)
	skus.Get("/code/:code", cfg.SKUHandler.GetBySKU)
	skus.Get("/:id", cfg.SKUHandler.GetByID)
	skus.Put("/:id", middleware.RequireRole("owner", "admin", "warehouse", "sales"), cfg.SKUHandler.Update)
	skus.Delete("/:id", middleware.RequireRole("owner", "admin", "accountant"), cfg.SKUHandler.Delete)

	// Bundle Routes
	bundles := protected.Group("/bundles")
	bundles.Get("/:sku", cfg.BundleHandler.GetComponents)
	bundles.Post("/:sku", middleware.RequireRole("owner", "admin", "warehouse"), cfg.BundleHandler.SetComponents)
	bundles.Get("/:sku/explode", cfg.BundleHandler.Explode)

	// Inventory / Stock Routes
	stocks := protected.Group("/inventory/stocks")
	stocks.Get("/", cfg.StockHandler.ListStock)
	stocks.Get("/:skuId", cfg.StockHandler.GetStock)
	stocks.Post("/adjust", middleware.RequireRole("owner", "admin", "warehouse"), cfg.StockHandler.Adjust)
	stocks.Get("/:skuId/movements", cfg.StockHandler.GetMovements)

	// Customer Routes
	customers := protected.Group("/customers")
	customers.Get("/", cfg.CustomerHandler.List)
	customers.Post("/", middleware.RequireRole("owner", "admin", "sales"), cfg.CustomerHandler.Create)
	customers.Get("/:id", cfg.CustomerHandler.GetByID)
	customers.Put("/:id", middleware.RequireRole("owner", "admin", "sales"), cfg.CustomerHandler.Update)
	customers.Delete("/:id", middleware.RequireRole("owner", "admin", "accountant"), cfg.CustomerHandler.Delete)

	// Order Routes
	orders := protected.Group("/orders")
	orders.Get("/", cfg.OrderHandler.List)
	orders.Post("/", middleware.RequireRole("owner", "admin", "sales"), cfg.OrderHandler.Create)
	orders.Get("/:id", cfg.OrderHandler.GetByID)
	orders.Post("/:id/ship", middleware.RequireRole("owner", "admin", "warehouse", "sales"), cfg.OrderHandler.Ship)
	orders.Post("/:id/cancel", middleware.RequireRole("owner", "admin", "sales"), cfg.OrderHandler.Cancel)

	// Purchasing & Supplier Routes
	suppliers := protected.Group("/suppliers")
	suppliers.Get("/", cfg.PurchasingHandler.ListSuppliers)
	suppliers.Post("/", middleware.RequireRole("owner", "admin", "warehouse", "accountant"), cfg.PurchasingHandler.CreateSupplier)
	suppliers.Get("/:id", cfg.PurchasingHandler.GetSupplierByID)

	pos := protected.Group("/purchases/orders")
	pos.Get("/", cfg.PurchasingHandler.ListPOs)
	pos.Post("/", middleware.RequireRole("owner", "admin", "warehouse", "accountant"), cfg.PurchasingHandler.CreatePO)
	pos.Get("/:id", cfg.PurchasingHandler.GetPOByID)
	pos.Post("/:id/approve", middleware.RequireRole("owner", "admin", "accountant"), cfg.PurchasingHandler.ApprovePO)
	pos.Post("/:id/receive", middleware.RequireRole("owner", "admin", "warehouse"), cfg.PurchasingHandler.ReceiveGoods)

	// Invoice Routes
	invoices := protected.Group("/invoices")
	invoices.Get("/", cfg.InvoiceHandler.List)
	invoices.Post("/", middleware.RequireRole("owner", "admin", "accountant", "sales"), cfg.InvoiceHandler.Create)
	invoices.Get("/:id", cfg.WorkspaceHandler.GetInvoiceByID)
	invoices.Post("/:id/pay", middleware.RequireRole("owner", "admin", "accountant"), cfg.InvoiceHandler.MarkAsPaid)
	invoices.Post("/:id/payment", middleware.RequireRole("owner", "admin", "accountant"), cfg.InvoiceHandler.MarkAsPaid)

	// Reporting Routes
	reports := protected.Group("/reports")
	reports.Get("/dashboard", cfg.ReportHandler.GetDashboard)
	reports.Get("/revenue", cfg.ReportHandler.GetRevenue)
	reports.Get("/financial-summary", cfg.ReportHandler.GetFinancialSummary)
	reports.Get("/inventory-valuation", cfg.ReportHandler.GetInventoryValuation)

	// Settings Routes
	settings := protected.Group("/settings")
	settings.Get("/", cfg.SettingsHandler.GetSettings)
	settings.Put("/", middleware.RequireRole("owner", "admin"), cfg.SettingsHandler.UpdateSettings)

	// Workspace compatibility routes for erp-web-v2
	ws := protected.Group("/workspace")
	ws.Get("/products", cfg.WorkspaceHandler.GetProducts)
	ws.Get("/orders", cfg.WorkspaceHandler.GetOrders)
	ws.Get("/invoices", cfg.WorkspaceHandler.GetInvoices)
	ws.Get("/customers", cfg.WorkspaceHandler.GetCustomers)
	ws.Get("/purchase-orders", cfg.WorkspaceHandler.GetPurchaseOrders)
	ws.Get("/quotations", cfg.WorkspaceHandler.GetQuotations)
	ws.Get("/goods-receives", cfg.WorkspaceHandler.GetGoodsReceives)
	ws.Get("/goods-issues", cfg.WorkspaceHandler.GetGoodsIssues)

	protected.Get("/products/:code", cfg.WorkspaceHandler.GetProductByCode)
	protected.Post("/products", middleware.RequireRole("owner", "admin", "warehouse", "sales"), cfg.WorkspaceHandler.CreateProduct)
	protected.Put("/products/:code/status", middleware.RequireRole("owner", "admin", "warehouse", "sales"), cfg.WorkspaceHandler.UpdateProductStatus)
	protected.Delete("/products/:code", middleware.RequireRole("owner", "admin", "accountant"), cfg.WorkspaceHandler.DeleteProduct)
	protected.Post("/sales-orders", middleware.RequireRole("owner", "admin", "sales"), cfg.WorkspaceHandler.CreateSalesOrder)
	protected.Get("/sales-orders/:id", cfg.WorkspaceHandler.GetSalesOrderByID)
	protected.Put("/sales-orders/:id/status", middleware.RequireRole("owner", "admin", "sales", "warehouse"), cfg.WorkspaceHandler.UpdateSalesOrderStatus)
	protected.Post("/invoices/from-so/:soRef", middleware.RequireRole("owner", "admin", "accountant", "sales"), cfg.WorkspaceHandler.CreateInvoiceFromSO)
	protected.Post("/stock-adjustments", middleware.RequireRole("owner", "admin", "warehouse"), cfg.WorkspaceHandler.AdjustStock)
	protected.Get("/bundle-components", cfg.WorkspaceHandler.GetBundleComponents)
	protected.Get("/bundle-components/:sku", cfg.WorkspaceHandler.GetBundleComponents)

	// Customer Workspace & Detail endpoints
	protected.Get("/customers/:id", cfg.WorkspaceHandler.GetCustomerByID)
	protected.Put("/customers/:id/status", middleware.RequireRole("owner", "admin", "sales"), cfg.WorkspaceHandler.UpdateCustomerStatus)

	// Quotation endpoints
	protected.Post("/quotations", middleware.RequireRole("owner", "admin", "sales"), cfg.WorkspaceHandler.CreateQuotation)
	protected.Get("/quotations/:id", cfg.WorkspaceHandler.GetQuotationByID)
	protected.Put("/quotations/:id/status", middleware.RequireRole("owner", "admin", "sales"), cfg.WorkspaceHandler.UpdateQuotationStatus)
	protected.Post("/quotations/:id/convert", middleware.RequireRole("owner", "admin", "sales"), cfg.WorkspaceHandler.ConvertQuotationToSO)

	// Purchase Orders REST endpoints for erp-web-v2
	protected.Get("/purchase-orders", cfg.WorkspaceHandler.GetPurchaseOrders)
	protected.Post("/purchase-orders", middleware.RequireRole("owner", "admin", "warehouse", "accountant"), cfg.WorkspaceHandler.CreatePurchaseOrder)
	protected.Get("/purchase-orders/:id", cfg.WorkspaceHandler.GetPurchaseOrderByID)
	protected.Put("/purchase-orders/:id/status", middleware.RequireRole("owner", "admin", "warehouse", "accountant"), cfg.WorkspaceHandler.UpdatePurchaseOrderStatus)

	// Goods Receives REST endpoints
	protected.Get("/goods-receives", cfg.WorkspaceHandler.GetGoodsReceives)
	protected.Post("/goods-receives", middleware.RequireRole("owner", "admin", "warehouse"), cfg.WorkspaceHandler.CreateGoodsReceive)
	protected.Get("/goods-receives/:id", cfg.WorkspaceHandler.GetGoodsReceiveByID)

	// Goods Issues REST endpoints
	protected.Get("/goods-issues", cfg.WorkspaceHandler.GetGoodsIssues)
	protected.Post("/goods-issues", middleware.RequireRole("owner", "admin", "warehouse"), cfg.WorkspaceHandler.CreateGoodsIssue)
	protected.Get("/goods-issues/:id", cfg.WorkspaceHandler.GetGoodsIssueByID)

	// Public TikTok Webhook & OAuth Callback (No JWT required)
	api.Get("/integrations/tiktok/callback", cfg.TikTokHandler.Callback)
	api.Post("/integrations/tiktok/webhook", cfg.TikTokHandler.ReceiveWebhook)

	// Compatibility aliases for /api/tiktok/callback and /api/tiktok/webhook
	cfg.App.Get("/api/tiktok/callback", cfg.TikTokHandler.Callback)
	cfg.App.Post("/api/tiktok/webhook", cfg.TikTokHandler.ReceiveWebhook)

	// Protected TikTok Integration Routes
	tiktok := protected.Group("/integrations/tiktok")
	tiktok.Get("/connection", cfg.TikTokHandler.GetConnection)
	tiktok.Post("/connect", middleware.RequireRole("owner", "admin"), cfg.TikTokHandler.StartConnect)
	tiktok.Post("/orders/sync", middleware.RequireRole("owner", "admin", "sales", "warehouse"), cfg.TikTokHandler.SyncOrders)
	tiktok.Get("/sync-runs", cfg.TikTokHandler.GetSyncRuns)
	tiktok.Get("/orders", cfg.TikTokHandler.GetOrders)
	tiktok.Get("/products", cfg.TikTokHandler.GetProducts)
	tiktok.Get("/stock-preview", cfg.TikTokHandler.GetStockSyncPreview)
	tiktok.Get("/mappings", cfg.TikTokHandler.ListMappings)
	tiktok.Post("/mappings", middleware.RequireRole("owner", "admin", "warehouse"), cfg.TikTokHandler.SaveMapping)
	tiktok.Post("/orders/manual-sync", middleware.RequireRole("owner", "admin", "sales", "warehouse"), cfg.TikTokHandler.SyncOrder)
	tiktok.Get("/logs", cfg.TikTokHandler.GetSyncLogs)
}
