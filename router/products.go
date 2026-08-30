package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
)

func RegisterProductRoutes(api fiber.Router) {
	// Products
	api.Get("/products", handlers.ListResource(func() interface{} { return &[]models.Product{} }))
	api.Get("/products/:sku", handlers.GetResource(func() interface{} { return &models.Product{} }, "sku", "sku"))
	api.Post("/products", middleware.RequireRoles("owner", "warehouse"), handlers.CreateProduct)
	api.Put("/products/:sku", middleware.RequireRoles("owner", "warehouse"), handlers.UpdateProduct)
	api.Delete("/products/:sku", middleware.RequireRoles("owner", "warehouse"), handlers.DeleteProduct)

	// Bundle Components
	api.Post("/bundle-components", middleware.RequireRoles("owner", "warehouse"), handlers.SetBundleComponents)
	api.Get("/bundle-components", handlers.ListResource(func() interface{} { return &[]models.BundleComponent{} }))
	api.Get("/bundle-components/:sku", handlers.ListResourceWhere(func() interface{} { return &[]models.BundleComponent{} }, "bundle_sku", "sku"))

	// BOMs
	api.Get("/boms", handlers.ListBOMs)
	api.Post("/boms", handlers.CreateBOM)
	api.Delete("/boms/:id", handlers.DeleteBOM)
	api.Get("/boms/:sku", handlers.GetBOM)
	api.Put("/boms/:sku", handlers.SaveBOM)
	api.Post("/boms/:sku/purchase-request", handlers.CreatePurchaseRequestFromBOM)
	api.Post("/boms/:sku/recalculate", handlers.RecalculateBOMCost)
	api.Post("/boms/:sku/duplicate", handlers.DuplicateBOM)
}
