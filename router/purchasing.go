package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
)

func RegisterPurchasingRoutes(api fiber.Router) {
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
	api.Get("/goods-receives", handlers.ListResource(func() interface{} { return &[]models.GoodsReceive{} }, "Items", "LandedCosts", "AuditTrail"))
	api.Get("/goods-receives/:id", handlers.GetResource(func() interface{} { return &models.GoodsReceive{} }, "id", "id", "Items", "LandedCosts", "AuditTrail"))
	api.Post("/goods-receives", handlers.CreateGoodsReceive)

	// Sampling
	api.Get("/sampling-campaigns", handlers.ListResource(func() interface{} { return &[]models.SamplingCampaign{} }, "Recipients"))
	api.Get("/sampling-campaigns/:id", handlers.GetResource(func() interface{} { return &models.SamplingCampaign{} }, "id", "id", "Recipients"))
	api.Post("/sampling-campaigns", handlers.CreateSamplingCampaign)
	api.Post("/sampling-campaigns/:id/recipients", handlers.AddSamplingRecipient)
	api.Put("/sampling-campaigns/:id/status", handlers.UpdateSamplingStatus)
}
