package route

import (
	"chawy-erp-api/internal/delivery/http/handler"
	"chawy-erp-api/internal/delivery/http/middleware"
	perm "chawy-erp-api/internal/domain/permission"

	"github.com/gofiber/fiber/v2"
)

type Config struct {
	App                   *fiber.App
	AuthHandler           *handler.AuthHandler
	SKUHandler            *handler.SKUHandler
	StockHandler          *handler.StockHandler
	BundleHandler         *handler.BundleHandler
	FormulaHandler        *handler.FormulaHandler
	CustomerHandler       *handler.CustomerHandler
	OrderHandler          *handler.OrderHandler
	PurchasingHandler     *handler.PurchasingHandler
	InvoiceHandler        *handler.InvoiceHandler
	ReportHandler         *handler.ReportHandler
	FinanceHandler        *handler.FinanceHandler
	WorkspaceHandler      *handler.WorkspaceHandler
	TikTokHandler         *handler.TikTokHandler
	SettingsHandler       *handler.SettingsHandler
	UploadHandler         *handler.UploadHandler
	LiveHandler           *handler.LiveHandler
	SalesReturnHandler    *handler.SalesReturnHandler
	ShopeeHandler         *handler.ShopeeHandler
	ShopeeInsightHandler  *handler.ShopeeInsightHandler
	SKUCostHistoryHandler *handler.SKUCostHistoryHandler
	CRMHandler            *handler.CRMHandler
	SalesAnalyticsHandler *handler.SalesAnalyticsHandler
	CRMIntegrationHandler *handler.CRMIntegrationHandler
	BoothHandler          *handler.BoothHandler
	StockOutHandler       *handler.StockOutHandler
	PermissionHandler     *handler.PermissionHandler
	// AccessChecker reads the owner-edited role permission matrix. When nil,
	// every matrix-guarded route is owner-only (fail closed).
	AccessChecker   middleware.AccessChecker
	CRMServiceToken       string
	JWTSecret             string
	UserStatusLoader      middleware.UserStatusLoader
}

func RegisterRoutes(cfg Config) {
	api := cfg.App.Group("/api/v1")

	// can guards a route with the role permission matrix; canAny passes when
	// the role holds any one of the rules (a route shared by two pages).
	can := func(page string, action perm.Action) fiber.Handler {
		return middleware.RequireAccess(cfg.AccessChecker, middleware.Rule{Page: page, Action: action})
	}
	canAny := func(rules ...middleware.Rule) fiber.Handler {
		return middleware.RequireAccess(cfg.AccessChecker, rules...)
	}
	rule := func(page string, action perm.Action) middleware.Rule {
		return middleware.Rule{Page: page, Action: action}
	}

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
	authGroup.Get("/me", middleware.AuthMiddleware(cfg.JWTSecret, cfg.UserStatusLoader), cfg.AuthHandler.Me)

	// Public Expense Upload Routes (for mobile quick upload & LINE OA integration)
	publicExpenses := api.Group("/public/expenses")
	publicExpenses.Post("/scan-receipt", cfg.FinanceHandler.ScanReceipt)
	publicExpenses.Post("/", cfg.FinanceHandler.CreateExpense)
	publicExpenses.Post("", cfg.FinanceHandler.CreateExpense)
	publicExpenses.Post("/scan", cfg.FinanceHandler.ScanReceipt)

	// Direct public scan alias for erp-expense-upload
	api.Post("/scan", cfg.FinanceHandler.ScanReceipt)

	// Public Income Upload Routes (for LINE OA / erp-expense-upload /income)
	publicIncomes := api.Group("/public/incomes")
	publicIncomes.Post("/scan-slip", cfg.FinanceHandler.ScanIncomeSlip)
	publicIncomes.Post("/", cfg.FinanceHandler.CreateIncome)
	publicIncomes.Post("", cfg.FinanceHandler.CreateIncome)

	// CRM backoffice integration (service token, not a staff JWT). Registered
	// before the protected group so its routes answer before AuthMiddleware.
	if cfg.CRMIntegrationHandler != nil {
		crmIntegration := api.Group("/integrations/crm", middleware.ServiceToken(cfg.CRMServiceToken))
		crmIntegration.Get("/skus", cfg.CRMIntegrationHandler.SearchSKUs)
		crmIntegration.Get("/stock", cfg.CRMIntegrationHandler.Stock)
		crmIntegration.Post("/goods-issues", cfg.CRMIntegrationHandler.CreateGoodsIssue)
	}

	// Protected Routes Group
	protected := api.Group("/", middleware.AuthMiddleware(cfg.JWTSecret, cfg.UserStatusLoader))

	// Auth routes (protected)
	protected.Post("/auth/change-password", cfg.AuthHandler.ChangePassword)

	// RBAC Matrix (API-01)
	protected.Get("/rbac/permission-matrix", cfg.AuthHandler.GetPermissionMatrix)

	// Role permission matrix: the owner sets, per role, which pages can be
	// opened and whether data can be created, edited or deleted there.
	if cfg.PermissionHandler != nil {
		protected.Get("/auth/permissions", cfg.PermissionHandler.MyPermissions)
		rolePerms := protected.Group("/role-permissions", middleware.RequireRole("owner"))
		rolePerms.Get("/", cfg.PermissionHandler.GetMatrix)
		rolePerms.Put("/:role", cfg.PermissionHandler.UpdateRole)
	}

	// User Management Routes (API-01, API-31) - Restricted to 'owner' and 'admin'
	users := protected.Group("/users", middleware.RequireRole("owner"))
	users.Get("/", cfg.AuthHandler.ListUsers)
	users.Get("/:id", cfg.AuthHandler.GetUserByID)
	users.Post("/", cfg.AuthHandler.CreateUser)
	users.Put("/:id", cfg.AuthHandler.UpdateUser)
	users.Put("/:id/status", cfg.AuthHandler.UpdateUserStatus)
	users.Delete("/:id", cfg.AuthHandler.DeleteUser)

	// SKU Routes
	skus := protected.Group("/skus")
	skus.Get("/", cfg.SKUHandler.List)
	skus.Post("/search", cfg.SKUHandler.List)
	skus.Post("/", can(perm.PageSKU, perm.Create), cfg.SKUHandler.Create)
	skus.Get("/code/:code", cfg.SKUHandler.GetBySKU)
	skus.Get("/:id", cfg.SKUHandler.GetByID)
	skus.Get("/:id/receipts", cfg.SKUHandler.GetReceiptHistory)
	skus.Put("/:id", can(perm.PageSKU, perm.Edit), cfg.SKUHandler.Update)
	skus.Delete("/:id", can(perm.PageSKU, perm.Delete), cfg.SKUHandler.Delete)

	// Inventory Formulas Routes (Replaces SKU-based bundles)
	if cfg.FormulaHandler != nil {
		formulas := protected.Group("/inventory-formulas")
		formulas.Get("/", cfg.FormulaHandler.List)
		formulas.Post("/search", cfg.FormulaHandler.List)
		formulas.Post("/", can(perm.PageInventory, perm.Create), cfg.FormulaHandler.Create)
		formulas.Get("/:code", cfg.FormulaHandler.GetByCode)
		formulas.Put("/:code", can(perm.PageInventory, perm.Edit), cfg.FormulaHandler.Update)
		formulas.Patch("/:code/status", can(perm.PageInventory, perm.Edit), cfg.FormulaHandler.ToggleStatus)
		formulas.Delete("/:code", can(perm.PageInventory, perm.Delete), cfg.FormulaHandler.Delete)

		// Root aliases matching /api/inventory-formulas
		rootFormulas := cfg.App.Group("/api/inventory-formulas", middleware.AuthMiddleware(cfg.JWTSecret, cfg.UserStatusLoader))
		rootFormulas.Get("/", cfg.FormulaHandler.List)
		rootFormulas.Post("/search", cfg.FormulaHandler.List)
		rootFormulas.Post("/", can(perm.PageInventory, perm.Create), cfg.FormulaHandler.Create)
		rootFormulas.Get("/:code", cfg.FormulaHandler.GetByCode)
		rootFormulas.Put("/:code", can(perm.PageInventory, perm.Edit), cfg.FormulaHandler.Update)
		rootFormulas.Patch("/:code/status", can(perm.PageInventory, perm.Edit), cfg.FormulaHandler.ToggleStatus)
		rootFormulas.Delete("/:code", can(perm.PageInventory, perm.Delete), cfg.FormulaHandler.Delete)
	}

	// Bundle Routes (Legacy support)
	bundles := protected.Group("/bundles")
	bundles.Get("/:sku", cfg.BundleHandler.GetComponents)
	bundles.Post("/:sku", can(perm.PageInventory, perm.Edit), cfg.BundleHandler.SetComponents)
	bundles.Get("/:sku/explode", cfg.BundleHandler.Explode)

	// Inventory / Stock Routes
	stocks := protected.Group("/inventory/stocks")
	stocks.Get("/", cfg.StockHandler.ListStock)
	stocks.Get("/:skuId", cfg.StockHandler.GetStock)
	stocks.Post("/adjust", can(perm.PageInventory, perm.Edit), cfg.StockHandler.Adjust)
	stocks.Get("/:skuId/movements", cfg.StockHandler.GetMovements)

	// Customer Routes
	customers := protected.Group("/customers")
	customers.Get("/", cfg.CustomerHandler.List)
	customers.Post("/search", cfg.CustomerHandler.List)
	customers.Post("/", can(perm.PageCustomers, perm.Create), cfg.CustomerHandler.Create)
	customers.Get("/:id", cfg.CustomerHandler.GetByID)
	customers.Put("/:id", can(perm.PageCustomers, perm.Edit), cfg.CustomerHandler.Update)
	customers.Delete("/:id", can(perm.PageCustomers, perm.Delete), cfg.CustomerHandler.Delete)

	// Order Routes
	orders := protected.Group("/orders")
	orders.Get("/", cfg.OrderHandler.List)
	orders.Post("/search", cfg.OrderHandler.List)
	orders.Post("/", can(perm.PageOrders, perm.Create), cfg.OrderHandler.Create)
	orders.Get("/:id", cfg.OrderHandler.GetByID)
	orders.Post("/:id/ship", can(perm.PageOrders, perm.Edit), cfg.OrderHandler.Ship)
	orders.Post("/:id/cancel", can(perm.PageOrders, perm.Delete), cfg.OrderHandler.Cancel)
	if cfg.SalesReturnHandler != nil {
		orders.Get("/:id/returnable", cfg.SalesReturnHandler.GetOrderReturnable)
	}

	// Sales Return Routes
	if cfg.SalesReturnHandler != nil {
		returns := protected.Group("/returns")
		returns.Get("/", cfg.SalesReturnHandler.List)
		returns.Post("/search", cfg.SalesReturnHandler.List)
		returns.Post("/", can(perm.PageReturns, perm.Create), cfg.SalesReturnHandler.Create)
		returns.Get("/:id", cfg.SalesReturnHandler.GetByID)
		returns.Put("/:id", can(perm.PageReturns, perm.Edit), cfg.SalesReturnHandler.Update)
		returns.Post("/:id/submit", can(perm.PageReturns, perm.Edit), cfg.SalesReturnHandler.Submit)
		// Approve / reject stay owner-only: approvals are not part of the matrix.
		returns.Post("/:id/approve", middleware.RequireRole("owner"), cfg.SalesReturnHandler.Approve)
		returns.Post("/:id/reject", middleware.RequireRole("owner"), cfg.SalesReturnHandler.Reject)
		returns.Post("/:id/complete", can(perm.PageReturns, perm.Edit), cfg.SalesReturnHandler.Complete)
		returns.Post("/:id/cancel", can(perm.PageReturns, perm.Delete), cfg.SalesReturnHandler.Cancel)
	}

	// Purchasing & Supplier Routes
	suppliers := protected.Group("/suppliers")
	suppliers.Get("/", cfg.PurchasingHandler.ListSuppliers)
	suppliers.Post("/search", cfg.PurchasingHandler.ListSuppliers)
	suppliers.Post("/", can(perm.PagePurchaseOrder, perm.Create), cfg.PurchasingHandler.CreateSupplier)
	suppliers.Get("/:id", cfg.PurchasingHandler.GetSupplierByID)

	pos := protected.Group("/purchases/orders")
	pos.Get("/", cfg.PurchasingHandler.ListPOs)
	pos.Post("/search", cfg.PurchasingHandler.ListPOs)
	pos.Post("/", can(perm.PagePurchaseOrder, perm.Create), cfg.PurchasingHandler.CreatePO)
	pos.Get("/:id", cfg.PurchasingHandler.GetPOByID)
	// PO approval keeps its fixed roles (approvals are not part of the matrix).
	pos.Post("/:id/approve", middleware.RequireRole("owner", "accountant"), cfg.PurchasingHandler.ApprovePO)
	pos.Post("/:id/receive", can(perm.PageGoodsReceive, perm.Create), cfg.PurchasingHandler.ReceiveGoods)

	// Invoice Routes
	invoices := protected.Group("/invoices")
	invoices.Get("/", cfg.InvoiceHandler.List)
	invoices.Post("/search", cfg.InvoiceHandler.List)
	invoices.Post("/", can(perm.PageInvoices, perm.Create), cfg.InvoiceHandler.Create)
	invoices.Get("/:id", cfg.WorkspaceHandler.GetInvoiceByID)
	invoices.Get("/:id/pdf", cfg.InvoiceHandler.ExportPDF)
	invoices.Post("/:id/pay", can(perm.PageInvoices, perm.Edit), cfg.InvoiceHandler.MarkAsPaid)
	invoices.Post("/:id/payment", can(perm.PageInvoices, perm.Edit), cfg.InvoiceHandler.MarkAsPaid)

	// Reporting Routes
	reports := protected.Group("/reports")
	reports.Get("/dashboard", cfg.ReportHandler.GetDashboard)
	reports.Get("/revenue", cfg.ReportHandler.GetRevenue)
	reports.Get("/financial-summary", cfg.ReportHandler.GetFinancialSummary)
	reports.Get("/inventory-valuation", cfg.ReportHandler.GetInventoryValuation)

	// Finance Routes (Restricted to owner and accountant)
	financeGroup := protected.Group("/finance")
	// The journal page links each entry to its expense and payment slip, so
	// expense reads are also open to roles that can view the journal.
	expenseRead := canAny(rule(perm.PageFinanceExpenses, perm.View), rule(perm.PageFinanceJournal, perm.View))
	journalRead := can(perm.PageFinanceJournal, perm.View)
	financeReports := can(perm.PageFinanceReports, perm.View)

	// Journal Entries
	financeGroup.Get("/journal-entries", journalRead, cfg.FinanceHandler.ListJournalEntries)
	financeGroup.Get("/journal-entries/:id", journalRead, cfg.FinanceHandler.GetJournalEntryByID)

	// Expenses
	financeGroup.Get("/expenses", expenseRead, cfg.FinanceHandler.ListExpenses)
	financeGroup.Get("/expenses/by-code/:code", expenseRead, cfg.FinanceHandler.GetExpenseByCode)
	financeGroup.Post("/expenses", can(perm.PageFinanceExpenses, perm.Create), cfg.FinanceHandler.CreateExpense)
	financeGroup.Post("/expenses/scan-receipt", can(perm.PageFinanceExpenses, perm.Create), cfg.FinanceHandler.ScanReceipt)
	financeGroup.Put("/expenses/:id", can(perm.PageFinanceExpenses, perm.Edit), cfg.FinanceHandler.UpdateExpense)
	financeGroup.Delete("/expenses/:id", can(perm.PageFinanceExpenses, perm.Delete), cfg.FinanceHandler.DeleteExpense)

	// Payment slips (expense / income).
	financeGroup.Get("/slips/:folder/:filename", expenseRead, cfg.FinanceHandler.GetSlip)

	// Financial Reports
	financeGroup.Get("/reports/general-ledger", financeReports, cfg.FinanceHandler.GetGeneralLedger)
	financeGroup.Get("/reports/trial-balance", financeReports, cfg.FinanceHandler.GetTrialBalance)
	financeGroup.Get("/reports/pnl", financeReports, cfg.FinanceHandler.GetProfitAndLoss)
	financeGroup.Get("/reports/revenue-by-channel", financeReports, cfg.FinanceHandler.GetRevenueByChannel)

	// Exports (.xlsx)
	financeGroup.Get("/export/journal", journalRead, cfg.FinanceHandler.ExportJournal)
	financeGroup.Get("/export/expenses", expenseRead, cfg.FinanceHandler.ExportExpenses)
	financeGroup.Get("/export/pnl", financeReports, cfg.FinanceHandler.ExportPnL)

	// Also register direct root aliases as planned in plan/FINANCE_MODULE.md
	protected.Get("/journal-entries", journalRead, cfg.FinanceHandler.ListJournalEntries)
	protected.Get("/journal-entries/:id", journalRead, cfg.FinanceHandler.GetJournalEntryByID)
	protected.Get("/expenses", expenseRead, cfg.FinanceHandler.ListExpenses)
	protected.Post("/expenses", can(perm.PageFinanceExpenses, perm.Create), cfg.FinanceHandler.CreateExpense)
	protected.Post("/expenses/scan-receipt", can(perm.PageFinanceExpenses, perm.Create), cfg.FinanceHandler.ScanReceipt)
	protected.Put("/expenses/:id", can(perm.PageFinanceExpenses, perm.Edit), cfg.FinanceHandler.UpdateExpense)
	protected.Delete("/expenses/:id", can(perm.PageFinanceExpenses, perm.Delete), cfg.FinanceHandler.DeleteExpense)
	protected.Get("/export/journal", journalRead, cfg.FinanceHandler.ExportJournal)
	protected.Get("/export/expenses", expenseRead, cfg.FinanceHandler.ExportExpenses)

	// Settings Routes
	settings := protected.Group("/settings")
	settings.Get("/", cfg.SettingsHandler.GetSettings)
	settings.Put("/", middleware.RequireRole("owner"), cfg.SettingsHandler.UpdateSettings)

	// Workspace compatibility routes for erp-web-v2
	ws := protected.Group("/workspace")
	ws.Get("/products", cfg.WorkspaceHandler.GetProducts)
	ws.Post("/products/search", cfg.WorkspaceHandler.GetProducts)
	ws.Get("/orders", cfg.WorkspaceHandler.GetOrders)
	ws.Post("/orders/search", cfg.WorkspaceHandler.GetOrders)
	ws.Get("/invoices", cfg.WorkspaceHandler.GetInvoices)
	ws.Post("/invoices/search", cfg.WorkspaceHandler.GetInvoices)
	ws.Get("/customers", cfg.WorkspaceHandler.GetCustomers)
	ws.Post("/customers/search", cfg.WorkspaceHandler.GetCustomers)
	ws.Get("/purchase-orders", cfg.WorkspaceHandler.GetPurchaseOrders)
	ws.Post("/purchase-orders/search", cfg.WorkspaceHandler.GetPurchaseOrders)
	ws.Get("/quotations", cfg.WorkspaceHandler.GetQuotations)
	ws.Post("/quotations/search", cfg.WorkspaceHandler.GetQuotations)
	ws.Get("/goods-receives", cfg.WorkspaceHandler.GetGoodsReceives)
	ws.Post("/goods-receives/search", cfg.WorkspaceHandler.GetGoodsReceives)
	ws.Get("/goods-issues", cfg.WorkspaceHandler.GetGoodsIssues)
	ws.Post("/goods-issues/search", cfg.WorkspaceHandler.GetGoodsIssues)

	protected.Post("/skus/resolve", cfg.WorkspaceHandler.ResolveSKUs)
	protected.Get("/products/:code", cfg.WorkspaceHandler.GetProductByCode)
	// Explicit ID-based product routes: :code above is always a SKU string,
	// so numeric SKUs can never be mistaken for record IDs (FULL-12 v2).
	protected.Get("/products/id/:id", cfg.WorkspaceHandler.GetProductByID)
	protected.Put("/products/id/:id", can(perm.PageSKU, perm.Edit), cfg.WorkspaceHandler.UpdateProductByID)
	protected.Put("/products/id/:id/status", can(perm.PageSKU, perm.Edit), cfg.WorkspaceHandler.UpdateProductStatusByID)
	protected.Put("/products/id/:id/reorder", can(perm.PageInventory, perm.Edit), cfg.WorkspaceHandler.UpdateProductReorderByID)
	protected.Delete("/products/id/:id", can(perm.PageSKU, perm.Delete), cfg.WorkspaceHandler.DeleteProductByID)
	protected.Post("/products/bulk-delete", can(perm.PageSKU, perm.Delete), cfg.WorkspaceHandler.BulkDeleteProducts)
	protected.Get("/products/template/download", cfg.WorkspaceHandler.DownloadProductTemplate)
	protected.Post("/products/import", can(perm.PageSKU, perm.Create), cfg.WorkspaceHandler.ImportProductsXLSX)
	// Compatibility aliases under /skus
	protected.Get("/skus/template/download", cfg.WorkspaceHandler.DownloadProductTemplate)
	protected.Post("/skus/import", can(perm.PageSKU, perm.Create), cfg.WorkspaceHandler.ImportProductsXLSX)
	protected.Post("/products", can(perm.PageSKU, perm.Create), cfg.WorkspaceHandler.CreateProduct)
	protected.Put("/products/:code", can(perm.PageSKU, perm.Edit), cfg.WorkspaceHandler.UpdateProduct)
	protected.Put("/products/:code/status", can(perm.PageSKU, perm.Edit), cfg.WorkspaceHandler.UpdateProductStatus)
	protected.Put("/products/:code/reorder", can(perm.PageInventory, perm.Edit), cfg.WorkspaceHandler.UpdateProductReorder)
	protected.Delete("/products/:code", can(perm.PageSKU, perm.Delete), cfg.WorkspaceHandler.DeleteProduct)
	protected.Post("/sales-orders", can(perm.PageOrders, perm.Create), cfg.WorkspaceHandler.CreateSalesOrder)
	protected.Get("/sales-orders/:id", cfg.WorkspaceHandler.GetSalesOrderByID)
	protected.Get("/sales-orders/:id/pdf", cfg.WorkspaceHandler.ExportSalesOrderPDF)
	protected.Put("/sales-orders/:id/status", can(perm.PageOrders, perm.Edit), cfg.WorkspaceHandler.UpdateSalesOrderStatus)
	protected.Post("/invoices/from-so/:soRef", can(perm.PageInvoices, perm.Create), cfg.WorkspaceHandler.CreateInvoiceFromSO)
	protected.Post("/stock-adjustments", can(perm.PageInventory, perm.Edit), cfg.WorkspaceHandler.AdjustStock)
	protected.Get("/bundle-components", cfg.WorkspaceHandler.GetBundleComponents)
	protected.Get("/bundle-components/:sku", cfg.WorkspaceHandler.GetBundleComponents)
	protected.Get("/sku-accessories", cfg.WorkspaceHandler.GetSKUAccessories)
	protected.Get("/sku-accessories/:sku", cfg.WorkspaceHandler.GetSKUAccessories)

	// Customer Workspace & Detail endpoints
	// FULL-30: the native /customers/:id above already serves GET detail with
	// the compatibility DTO response; the duplicate registration that shadowed
	// it (and returned raw domain JSON) has been removed.
	protected.Put("/customers/:id/status", can(perm.PageCustomers, perm.Edit), cfg.WorkspaceHandler.UpdateCustomerStatus)

	// Quotation endpoints
	protected.Post("/quotations", can(perm.PageQuotation, perm.Create), cfg.WorkspaceHandler.CreateQuotation)
	protected.Get("/quotations/:id", cfg.WorkspaceHandler.GetQuotationByID)
	protected.Get("/quotations/:id/pdf", cfg.WorkspaceHandler.ExportQuotationPDF)
	protected.Put("/quotations/:id/status", can(perm.PageQuotation, perm.Edit), cfg.WorkspaceHandler.UpdateQuotationStatus)
	protected.Post("/quotations/:id/convert", can(perm.PageQuotation, perm.Edit), cfg.WorkspaceHandler.ConvertQuotationToSO)

	// Purchase Orders REST endpoints for erp-web-v2
	protected.Get("/purchase-orders", cfg.WorkspaceHandler.GetPurchaseOrders)
	protected.Post("/purchase-orders/search", cfg.WorkspaceHandler.GetPurchaseOrders)
	protected.Post("/purchase-orders", can(perm.PagePurchaseOrder, perm.Create), cfg.WorkspaceHandler.CreatePurchaseOrder)
	protected.Get("/purchase-orders/:id", cfg.WorkspaceHandler.GetPurchaseOrderByID)
	protected.Put("/purchase-orders/:id/status", can(perm.PagePurchaseOrder, perm.Edit), cfg.WorkspaceHandler.UpdatePurchaseOrderStatus)

	// Goods Receives REST endpoints
	protected.Get("/goods-receives", cfg.WorkspaceHandler.GetGoodsReceives)
	protected.Post("/goods-receives", can(perm.PageGoodsReceive, perm.Create), cfg.WorkspaceHandler.CreateGoodsReceive)
	protected.Get("/goods-receives/:id", cfg.WorkspaceHandler.GetGoodsReceiveByID)

	// Upload Routes (Image upload max 5MB, png/jpg/jpeg)
	protected.Post("/upload/image", cfg.UploadHandler.UploadImage)

	// Goods Issues REST endpoints
	protected.Get("/goods-issues", cfg.WorkspaceHandler.GetGoodsIssues)
	protected.Post("/goods-issues", can(perm.PageGoodsIssue, perm.Create), cfg.WorkspaceHandler.CreateGoodsIssue)
	protected.Get("/goods-issues/:id", cfg.WorkspaceHandler.GetGoodsIssueByID)

	// Booth point of sale: every bill takes its stock out of the inventory
	// (warehouse 1) in one transaction; a void puts it back.
	if cfg.BoothHandler != nil {
		booth := protected.Group("/booth", can(perm.PageBooth, perm.View))
		booth.Get("/catalog", cfg.BoothHandler.GetCatalog)
		booth.Put("/config", middleware.RequireRole("owner"), cfg.BoothHandler.UpdateConfig)
		booth.Post("/quote", cfg.BoothHandler.Quote)
		booth.Get("/sales", cfg.BoothHandler.ListSales)
		booth.Post("/sales", can(perm.PageBooth, perm.Create), cfg.BoothHandler.CreateSale)
		booth.Post("/sales/:id/void", can(perm.PageBooth, perm.Edit), cfg.BoothHandler.VoidSale)
	}

	// Daily stock-out count: packers save what they pick as batches; the
	// server takes them out lot by lot (FEFO). Who may count and void older
	// batches is set in the config and checked in the usecase.
	if cfg.StockOutHandler != nil {
		so := protected.Group("/stock-outs", can(perm.PageStockOut, perm.View))
		so.Get("/config", cfg.StockOutHandler.GetConfig)
		so.Put("/config", middleware.RequireRole("owner"), cfg.StockOutHandler.SaveConfig)
		so.Get("/summary", cfg.StockOutHandler.Summary)
		so.Get("/history", cfg.StockOutHandler.History)
		so.Get("/stream", cfg.StockOutHandler.Stream)
		so.Get("/", cfg.StockOutHandler.List)
		so.Post("/", can(perm.PageStockOut, perm.Create), cfg.StockOutHandler.Create)
		so.Post("/:id/void", can(perm.PageStockOut, perm.Edit), cfg.StockOutHandler.Void)
		so.Post("/:id/review", middleware.RequireRole("owner"), cfg.StockOutHandler.Review)

		// The paths named in the stock-out spec. Guarded per route, not with
		// a "/stock" group: Fiber group middleware matches by string prefix
		// and would also guard /stock-adjustments.
		protected.Get("/stock/summary", can(perm.PageStockOut, perm.View), cfg.StockOutHandler.Summary)
		protected.Get("/stock/stream", can(perm.PageStockOut, perm.View), cfg.StockOutHandler.Stream)
	}

	// Public TikTok Webhook & OAuth Callback (No JWT required)
	api.Get("/integrations/tiktok/callback", cfg.TikTokHandler.Callback)
	api.Post("/integrations/tiktok/webhook", cfg.TikTokHandler.ReceiveWebhook)

	// Compatibility aliases for /api/tiktok/callback and /api/tiktok/webhook
	cfg.App.Get("/api/tiktok/callback", cfg.TikTokHandler.Callback)
	cfg.App.Post("/api/tiktok/webhook", cfg.TikTokHandler.ReceiveWebhook)

	// Protected TikTok Integration Routes
	tiktok := protected.Group("/integrations/tiktok")
	tiktok.Get("/connection", cfg.TikTokHandler.GetConnection)
	tiktok.Post("/connect", middleware.RequireRole("owner"), cfg.TikTokHandler.StartConnect)
	tiktok.Post("/orders/sync", can(perm.PageTikTokOrders, perm.Edit), cfg.TikTokHandler.SyncOrders)
	tiktok.Post("/orders/provinces/backfill", can(perm.PageTikTokOrders, perm.Edit), cfg.TikTokHandler.BackfillProvinces)
	tiktok.Get("/sync-runs", cfg.TikTokHandler.GetSyncRuns)
	tiktok.Get("/orders", cfg.TikTokHandler.GetOrders)
	tiktok.Post("/orders/search", cfg.TikTokHandler.GetOrders)
	tiktok.Get("/products", cfg.TikTokHandler.GetProducts)
	tiktok.Get("/stock-preview", cfg.TikTokHandler.GetStockSyncPreview)
	tiktok.Get("/mappings", cfg.TikTokHandler.ListMappings)
	tiktok.Post("/mappings", can(perm.PageTikTokOrders, perm.Create), cfg.TikTokHandler.SaveMapping)
	tiktok.Post("/orders/manual-sync", can(perm.PageTikTokOrders, perm.Edit), cfg.TikTokHandler.SyncOrder)
	tiktok.Get("/logs", cfg.TikTokHandler.GetSyncLogs)

	// Live & Content Routes
	liveGroup := protected.Group("/live")
	liveCreate := canAny(rule(perm.PageTikTokLive, perm.Create), rule(perm.PageMyLive, perm.Create))
	liveEdit := canAny(rule(perm.PageTikTokLive, perm.Edit), rule(perm.PageMyLive, perm.Edit))
	liveGroup.Get("/sessions", cfg.LiveHandler.ListSessions)
	liveGroup.Post("/sessions/search", cfg.LiveHandler.ListSessions)
	liveGroup.Post("/sessions", liveCreate, cfg.LiveHandler.CreateSession)
	liveGroup.Get("/sessions/:id", cfg.LiveHandler.GetSessionByID)
	liveGroup.Put("/sessions/:id", liveEdit, cfg.LiveHandler.UpdateSession)
	liveGroup.Post("/sessions/:id/approve", middleware.RequireRole("owner"), cfg.LiveHandler.ApproveSession)
	liveGroup.Post("/sessions/:id/reject", middleware.RequireRole("owner"), cfg.LiveHandler.RejectSession)
	// Wages: the streamer sees their own page; accountants read it with the
	// finance reports right.
	liveGroup.Get("/payroll", canAny(rule(perm.PageMyLive, perm.View), rule(perm.PageFinanceReports, perm.View)), cfg.LiveHandler.GetPayrollSummary)

	// Content Items (Schedule + Performance)
	liveGroup.Get("/content", cfg.LiveHandler.ListContentItems)
	liveGroup.Post("/content/search", cfg.LiveHandler.ListContentItems)
	liveGroup.Post("/content", liveCreate, cfg.LiveHandler.CreateContentItem)
	liveGroup.Put("/content/:id", liveEdit, cfg.LiveHandler.UpdateContentItem)
	liveGroup.Delete("/content/:id", can(perm.PageTikTokLive, perm.Delete), cfg.LiveHandler.DeleteContentItem)

	// Shopee Routes
	shopeeGroup := protected.Group("/shopee")
	shopeeGroup.Post("/orders/preview", cfg.ShopeeHandler.PreviewOrders)
	shopeeGroup.Post("/orders/import", can(perm.PageShopeeOrders, perm.Create), cfg.ShopeeHandler.ImportOrders)
	// Correct orders imported by the old parser (wrong order date / quantity) in place.
	shopeeGroup.Post("/orders/repair/preview", middleware.RequireRole("owner", "accountant"), cfg.ShopeeHandler.PreviewOrderRepair)
	shopeeGroup.Post("/orders/repair/apply", middleware.RequireRole("owner", "accountant"), cfg.ShopeeHandler.ApplyOrderRepair)
	shopeeGroup.Get("/orders", cfg.ShopeeHandler.GetOrders)
	shopeeGroup.Post("/orders/search", cfg.ShopeeHandler.GetOrders)
	shopeeGroup.Get("/orders/provinces", cfg.ShopeeHandler.GetProvinces)
	shopeeGroup.Get("/orders/:id", cfg.ShopeeHandler.GetOrderByID)
	shopeeGroup.Patch("/orders/items/:id/sku", can(perm.PageShopeeOrders, perm.Edit), cfg.ShopeeHandler.UpdateItemSKU)
	shopeeGroup.Delete("/orders/:id", can(perm.PageShopeeOrders, perm.Delete), cfg.ShopeeHandler.DeleteOrder)

	shopeeGroup.Post("/income/preview", cfg.ShopeeHandler.PreviewIncome)
	shopeeGroup.Post("/income/import", can(perm.PageShopeeIncome, perm.Create), cfg.ShopeeHandler.ImportIncome)
	shopeeGroup.Get("/income", cfg.ShopeeHandler.GetIncomes)
	shopeeGroup.Post("/income/search", cfg.ShopeeHandler.GetIncomes)
	shopeeGroup.Delete("/income/:id", can(perm.PageShopeeIncome, perm.Delete), cfg.ShopeeHandler.DeleteIncome)

	shopeeGroup.Get("/matching", cfg.ShopeeHandler.GetMatching)
	shopeeGroup.Get("/matching/export", cfg.ShopeeHandler.ExportMatchingCSV)
	shopeeGroup.Get("/dashboard", cfg.ShopeeHandler.GetDashboard)

	// Sales insights by order date (Chawy shop only; totals, no buyer data).
	if cfg.ShopeeInsightHandler != nil {
		insights := shopeeGroup.Group("/insights", can(perm.PageShopeeInsights, perm.View))
		insights.Get("/summary", cfg.ShopeeInsightHandler.Summary)
		insights.Get("/products", cfg.ShopeeInsightHandler.Products)
		insights.Get("/sets", cfg.ShopeeInsightHandler.Sets)
		insights.Get("/customers", cfg.ShopeeInsightHandler.Customers)
		insights.Get("/journey", cfg.ShopeeInsightHandler.Journey)
	}

	// SKU Cost History Routes
	protected.Post("/skus/:sku/cost-history", middleware.RequireRole("owner", "accountant"), cfg.SKUCostHistoryHandler.AddCostHistory)
	protected.Get("/skus/:sku/cost-history", cfg.SKUCostHistoryHandler.GetCostHistory)
	protected.Delete("/skus/cost-history/:id", middleware.RequireRole("owner", "accountant"), cfg.SKUCostHistoryHandler.DeleteCostHistory)

	// CRM Routes
	crmGroup := protected.Group("/crm")
	crmGroup.Get("/tiktok/provinces", can(perm.PageCRMProvinces, perm.View), cfg.CRMHandler.GetTiktokProvinceReport)
	crmGroup.Post("/tiktok/provinces/search", can(perm.PageCRMProvinces, perm.View), cfg.CRMHandler.SearchProvinceReport)
	// Order facts for the CRM sales dashboard; crm-core computes every KPI.
	crmGroup.Get("/sales-analytics", middleware.RequireRole("owner", "sales", "accountant"), cfg.SalesAnalyticsHandler.GetSalesAnalytics)

}
