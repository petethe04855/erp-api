package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
)

func RegisterOrderRoutes(api fiber.Router) {
	// Quotations
	api.Get("/quotations", handlers.ListResource(func() interface{} { return &[]models.Quotation{} }, "Lines"))
	api.Get("/quotations/:id", handlers.GetResource(func() interface{} { return &models.Quotation{} }, "id", "id", "Lines"))
	api.Post("/quotations", handlers.CreateQuotation)
	api.Put("/quotations/:id/status", handlers.UpdateQuotationStatus)
	api.Put("/quotations/:id/lead-source", handlers.UpdateQuotationLeadSource)
	api.Post("/quotations/:id/convert", handlers.ConvertQuotationToSalesOrder)

	// Sales Orders
	api.Get("/sales-orders", handlers.ListResource(func() interface{} { return &[]models.SalesOrder{} }, "Lines", "Lines.Allocations", "AuditTrail"))
	api.Get("/sales-orders/:id", handlers.GetResource(func() interface{} { return &models.SalesOrder{} }, "id", "id", "Lines", "Lines.Allocations", "AuditTrail"))
	api.Post("/sales-orders", handlers.CreateSalesOrder)
	api.Put("/sales-orders/:id/status", handlers.UpdateSalesOrderStatus)

	// Invoices
	api.Get("/invoices", handlers.ListResource(func() interface{} { return &[]models.Invoice{} }, "Lines", "AuditTrail"))
	api.Get("/invoices/:id", handlers.GetResource(func() interface{} { return &models.Invoice{} }, "id", "id", "Lines", "AuditTrail"))
	api.Post("/invoices", handlers.CreateInvoice)
	api.Post("/invoices/from-so/:soId", handlers.CreateInvoiceFromSO)
	api.Post("/invoices/:id/payment", handlers.RecordPayment)

	// Manual Orders
	api.Get("/manual-orders", handlers.ListResource(func() interface{} { return &[]models.ManualOrder{} }))
	api.Get("/manual-orders/:id", handlers.GetResource(func() interface{} { return &models.ManualOrder{} }, "id", "id"))
	api.Post("/manual-orders", handlers.CreateManualOrder)
	api.Put("/manual-orders/:id", handlers.UpdateManualOrder)
	api.Delete("/manual-orders/:id", handlers.DeleteManualOrder)

	// Customers
	api.Get("/customers", handlers.ListResource(func() interface{} { return &[]models.Customer{} }))
	api.Get("/customers/:id", handlers.GetResource(func() interface{} { return &models.Customer{} }, "id", "id"))
	api.Post("/customers", handlers.CreateCustomer)
	api.Put("/customers/:id", handlers.UpdateCustomer)
	api.Delete("/customers/:id", handlers.DeleteCustomer)
}
