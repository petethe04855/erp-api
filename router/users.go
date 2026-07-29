package router

import (
	"chawy-erp-api/handlers"
	"chawy-erp-api/middleware"
	"chawy-erp-api/models"

	"github.com/gofiber/fiber/v2"
)

func RegisterUserRoutes(api fiber.Router) {
	api.Get("/users", middleware.RequireRoles("owner"), handlers.ListResource(func() interface{} { return &[]models.AppUser{} }))
	api.Get("/users/:id", middleware.RequireRoles("owner"), handlers.GetResource(func() interface{} { return &models.AppUser{} }, "id", "id"))
	api.Post("/users", middleware.RequireRoles("owner"), handlers.CreateUser)
	api.Put("/users/:id", middleware.RequireRoles("owner"), handlers.UpdateUser)
	api.Put("/users/:id/status", middleware.RequireRoles("owner"), handlers.UpdateUserStatus)
	api.Delete("/users/:id", middleware.RequireRoles("owner"), handlers.DeleteUser)
}
