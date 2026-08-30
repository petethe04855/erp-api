package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterAuthRoutes(app *fiber.App) {
	app.Post("/api/auth/login", handlers.Login)
	app.Get("/api/auth/me", middleware.AuthRequired, handlers.GetCurrentUser)
	// TikTok sends the OAuth user-agent back here, so this route deliberately
	// remains public. The state cookie is verified by the handler.
	app.Get("/api/tiktok/callback", handlers.TiktokCallback)
	app.Post("/api/tiktok/webhook", handlers.ReceiveTiktokWebhook)
}
