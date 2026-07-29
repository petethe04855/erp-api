package router

import (
	"chawy-erp-api/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	// Public & Auth Routes
	RegisterAuthRoutes(app)

	// Protected API Routes Group
	api := app.Group("/api", middleware.AuthRequired)

	// Sub-router groups
	RegisterUserRoutes(api)
	RegisterProductRoutes(api)
	RegisterOrderRoutes(api)
	RegisterPurchasingRoutes(api)
	RegisterInventoryRoutes(api)
	RegisterFinanceRoutes(api)
	RegisterPlatformRoutes(api)
}
