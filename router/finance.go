package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
)

func RegisterFinanceRoutes(api fiber.Router) {
	// Accounting core
	api.Get("/account-mappings", middleware.RequirePermission("View"), handlers.ListAccountMappings)
	api.Put("/account-mappings/:key", middleware.RequirePermission("Edit"), handlers.UpsertAccountMapping)
	api.Get("/accounts", handlers.ListResource(func() interface{} { return &[]models.Account{} }))
	api.Get("/journal-entries", handlers.ListResource(func() interface{} { return &[]models.JournalEntry{} }, "Lines"))
	api.Get("/journal-entries/:id", handlers.GetResource(func() interface{} { return &models.JournalEntry{} }, "id", "id", "Lines"))
	api.Get("/customer-payments", handlers.ListResource(func() interface{} { return &[]models.CustomerPayment{} }))
	api.Get("/credit-notes", handlers.ListResource(func() interface{} { return &[]models.CreditNote{} }))
	api.Put("/credit-notes/:id/reverse", middleware.RequireRoles("owner", "accountant"), handlers.ReverseCreditNote)
	api.Get("/audit-logs", middleware.RequireRoles("owner", "accountant"), handlers.ListResource(func() interface{} { return &[]models.AuditLog{} }))
	api.Get("/reports/general-ledger", handlers.GeneralLedgerReport)
	api.Get("/reports/trial-balance", handlers.TrialBalanceReport)
	api.Get("/reports/inventory-valuation", handlers.InventoryValuationReport)
	api.Get("/reports/financial-summary", handlers.FinancialSummaryReport)
	api.Get("/reports/revenue", handlers.RevenueReport)
	api.Get("/integrity-runs", middleware.RequireRoles("owner", "accountant"), handlers.ListResource(func() interface{} { return &[]models.IntegrityRun{} }))
	api.Get("/integrity-issues", middleware.RequireRoles("owner", "accountant"), handlers.ListResource(func() interface{} { return &[]models.IntegrityIssue{} }))
	api.Post("/integrity-runs", middleware.RequireRoles("owner", "accountant"), handlers.RunIntegrityNow)
	api.Put("/integrity-issues/:id/resolve", middleware.RequireRoles("owner", "accountant"), handlers.ResolveIntegrityIssue)

	// Finance & Expenses
	api.Get("/expenses", handlers.ListResource(func() interface{} { return &[]models.Expense{} }))
	api.Get("/expenses/:id", handlers.GetResource(func() interface{} { return &models.Expense{} }, "id", "id"))
	api.Post("/expenses", middleware.RequireRoles("owner", "accountant"), handlers.CreateExpense)
	api.Put("/expenses/:id", middleware.RequireRoles("owner", "accountant"), handlers.UpdateExpense)
	api.Delete("/expenses/:id", middleware.RequireRoles("owner", "accountant"), handlers.DeleteExpense)

	// Budgets
	api.Get("/budgets", handlers.ListResource(func() interface{} { return &[]models.MonthBudget{} }))
	api.Get("/budgets/:id", handlers.GetResource(func() interface{} { return &models.MonthBudget{} }, "id", "id"))
	api.Post("/budgets", middleware.RequireRoles("owner", "accountant"), handlers.UpsertBudget)
	api.Put("/budgets/:id", middleware.RequireRoles("owner", "accountant"), handlers.UpdateBudget)
	api.Delete("/budgets/:id", middleware.RequireRoles("owner", "accountant"), handlers.DeleteBudget)

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
}
