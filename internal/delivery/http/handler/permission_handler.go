package handler

import (
	"chawy-erp-api/internal/delivery/http/dto"
	usecasePermission "chawy-erp-api/internal/usecase/permission"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type PermissionHandler struct {
	usecase usecasePermission.Usecase
}

func NewPermissionHandler(usecase usecasePermission.Usecase) *PermissionHandler {
	return &PermissionHandler{usecase: usecase}
}

// GetMatrix returns every configurable page and each role's access (owner only).
func (h *PermissionHandler) GetMatrix(c *fiber.Ctx) error {
	m, err := h.usecase.GetMatrix(c.UserContext())
	if err != nil {
		return err
	}
	return response.OK(c, dto.NewPermissionMatrixResponse(m))
}

// UpdateRole saves the access of one role (owner only).
func (h *PermissionHandler) UpdateRole(c *fiber.Ctx) error {
	var req dto.UpdateRolePermissionsRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}
	updatedBy, _ := c.Locals("email").(string)
	role := c.Params("role")

	ra, err := h.usecase.UpdateRole(c.UserContext(), role, req.ToRoleAccess(), updatedBy)
	if err != nil {
		return err
	}
	return response.OK(c, dto.RolePermissionsResponse{Role: role, Permissions: dto.NewRoleAccessDTO(ra)}, "บันทึกสิทธิ์เรียบร้อยแล้ว")
}

// MyPermissions returns the signed-in user's access, for menus and page guards.
func (h *PermissionHandler) MyPermissions(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	ra, err := h.usecase.GetRoleAccess(c.UserContext(), role)
	if err != nil {
		return err
	}
	return response.OK(c, dto.RolePermissionsResponse{Role: role, Permissions: dto.NewRoleAccessDTO(ra), Paths: dto.PagePaths()})
}
