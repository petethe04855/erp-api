package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
)

func RegisterFinanceRoutes(api fiber.Router) {
	// Accounting core
	api.Get("/accounts", handlers.ListResource(func() interface{} { return &[]models.Account{} }))
	api.Get("/journal-entries", handlers.ListResource(func() interface{} { return &[]models.JournalEntry{} }, "Lines"))
	api.Get("/journal-entries/:id", handlers.GetResource(func() interface{} { return &models.JournalEntry{} }, "id", "id", "Lines"))
	api.Get("/customer-payments", handlers.ListResource(func() interface{} { return &[]models.CustomerPayment{} }))
	api.Get("/credit-notes", handlers.ListResource(func() interface{} { return &[]models.CreditNote{} }))

	// Finance & Expenses
	api.Get("/expenses", handlers.ListResource(func() interface{} { return &[]models.Expense{} }))
	api.Get("/expenses/:id", handlers.GetResource(func() interface{} { return &models.Expense{} }, "id", "id"))
	api.Post("/expenses", handlers.CreateExpense)
	api.Put("/expenses/:id", handlers.UpdateExpense)
	api.Delete("/expenses/:id", middleware.RequireRoles("owner", "accountant"), handlers.DeleteExpense)

	// Budgets
	api.Get("/budgets", handlers.ListResource(func() interface{} { return &[]models.MonthBudget{} }))
	api.Get("/budgets/:id", handlers.GetResource(func() interface{} { return &models.MonthBudget{} }, "id", "id"))
	api.Post("/budgets", handlers.UpsertBudget)
	api.Put("/budgets/:id", handlers.UpdateBudget)
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
