package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
)

func RegisterPlatformRoutes(api fiber.Router) {
	// Platform & TikTok Connection
	api.Get("/tiktok/connection", handlers.GetTiktokConnection)
	api.Get("/tiktok/sync-runs", middleware.RequireRoles("owner", "warehouse", "accountant"), handlers.ListTiktokSyncRuns)
	api.Get("/tiktok/settlement-reconciliation", middleware.RequireRoles("owner", "accountant"), handlers.TiktokSettlementReconciliation)
	api.Post("/tiktok/connect", middleware.RequireRoles("owner"), handlers.StartTiktokConnect)
	api.Get("/tiktok/products", handlers.GetTiktokProducts)
	api.Post("/tiktok/products/import", middleware.RequireRoles("owner", "warehouse"), handlers.ImportTiktokProducts)
	api.Get("/tiktok/stock-sync-preview", middleware.RequireRoles("owner", "warehouse", "accountant"), handlers.TiktokStockSyncPreview)
	api.Post("/tiktok/orders/sync", middleware.RequireRoles("owner", "warehouse", "accountant"), handlers.SyncTiktokOrders)
	api.Get("/tiktok-orders", handlers.ListResource(func() interface{} { return &[]models.TiktokOrder{} }, "Items"))
	api.Get("/tiktok-orders/:id", handlers.GetResource(func() interface{} { return &models.TiktokOrder{} }, "id", "id", "Items"))
	api.Post("/tiktok-orders", middleware.RequireRoles("owner", "sales", "accountant"), handlers.CreateTiktokOrder)
	api.Put("/tiktok-orders/:id/imported", middleware.RequireRoles("owner", "sales", "accountant"), handlers.MarkTiktokOrderImported)
	api.Post("/tiktok-orders/:id/settle", middleware.RequireRoles("owner", "accountant"), handlers.ApplyTiktokSettlement)

	// Live Sessions
	api.Get("/live-sessions", handlers.ListResource(func() interface{} { return &[]models.LiveSession{} }))
	api.Get("/live-sessions/:id", handlers.GetResource(func() interface{} { return &models.LiveSession{} }, "id", "id"))
	api.Post("/live-sessions", handlers.CreateLiveSession)
	api.Put("/live-sessions/:id/status", handlers.UpdateLiveSessionStatus)

	// Content Schedule
	api.Get("/content-schedule", handlers.ListResource(func() interface{} { return &[]models.ContentScheduleItem{} }))
	api.Get("/content-schedule/:id", handlers.GetResource(func() interface{} { return &models.ContentScheduleItem{} }, "id", "id"))
	api.Post("/content-schedule", handlers.CreateContentSchedule)
	api.Put("/content-schedule/:id", handlers.UpdateContentSchedule)
	api.Put("/content-schedule/:id/status", handlers.UpdateContentScheduleStatus)
	api.Delete("/content-schedule/:id", handlers.DeleteContentSchedule)
}
