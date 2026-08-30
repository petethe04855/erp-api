package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
)

func RegisterInventoryRoutes(api fiber.Router) {
	// Stock Operations
	api.Get("/goods-issues", handlers.ListResource(func() interface{} { return &[]models.GoodsIssue{} }))
	api.Get("/goods-issues/:id", handlers.GetResource(func() interface{} { return &models.GoodsIssue{} }, "id", "id"))
	api.Post("/goods-issues", middleware.RequireRoles("owner", "warehouse"), handlers.CreateGoodsIssue)

	api.Get("/production-runs", handlers.ListResource(func() interface{} { return &[]models.ProductionRun{} }))
	api.Get("/production-runs/:id", handlers.GetResource(func() interface{} { return &models.ProductionRun{} }, "id", "id"))
	api.Post("/production-runs", handlers.CreateProductionRun)

	api.Get("/stock-returns", handlers.ListResource(func() interface{} { return &[]models.StockReturn{} }, "Allocations"))
	api.Get("/stock-returns/:id", handlers.GetResource(func() interface{} { return &models.StockReturn{} }, "id", "id", "Allocations"))
	api.Post("/stock-returns", middleware.RequireRoles("owner", "sales"), handlers.CreateStockReturn)
	api.Put("/stock-returns/:id/status", middleware.RequirePermission("Approve"), handlers.UpdateStockReturnStatus)

	api.Get("/stock-adjustments", handlers.ListResource(func() interface{} { return &[]models.StockAdjustment{} }, "Items"))
	api.Get("/stock-adjustments/:id", handlers.GetResource(func() interface{} { return &models.StockAdjustment{} }, "id", "id", "Items"))
	api.Post("/stock-adjustments", middleware.RequireRoles("owner", "warehouse"), handlers.CreateStockAdjustment)

	api.Get("/stock-transfers", handlers.ListResource(func() interface{} { return &[]models.StockTransfer{} }))
	api.Get("/stock-transfers/:id", handlers.GetResource(func() interface{} { return &models.StockTransfer{} }, "id", "id"))
	api.Post("/stock-transfers", middleware.RequireRoles("owner", "warehouse"), handlers.CreateStockTransfer)

	api.Get("/stock-lots", handlers.ListResource(func() interface{} { return &[]models.StockLot{} }))
	api.Get("/stock-lots/:id", handlers.GetResource(func() interface{} { return &models.StockLot{} }, "id", "id"))

	api.Get("/stock-movements", handlers.ListResource(func() interface{} { return &[]models.StockMovement{} }))
	api.Get("/stock-movements/:id", handlers.GetResource(func() interface{} { return &models.StockMovement{} }, "id", "id"))
}
