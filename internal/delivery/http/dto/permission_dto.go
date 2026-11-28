package dto

import (
	domainPermission "chawy-erp-api/internal/domain/permission"
	usecasePermission "chawy-erp-api/internal/usecase/permission"
)

// AccessDTO is one role's rights on one page.
type AccessDTO struct {
	View   bool `json:"view"`
	Create bool `json:"create"`
	Edit   bool `json:"edit"`
	Delete bool `json:"delete"`
}

// PermissionPageResponse describes a configurable page.
type PermissionPageResponse struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Group   string   `json:"group"`
	Path    string   `json:"path"`
	Actions []string `json:"actions"`
}

// PermissionMatrixResponse is GET /role-permissions.
type PermissionMatrixResponse struct {
	Roles       []string                        `json:"roles"`
	Pages       []PermissionPageResponse        `json:"pages"`
	Permissions map[string]map[string]AccessDTO `json:"permissions"`
}

// UpdateRolePermissionsRequest is PUT /role-permissions/:role.
type UpdateRolePermissionsRequest struct {
	Permissions map[string]AccessDTO `json:"permissions"`
}

// RolePermissionsResponse is the access of one role (also GET /auth/permissions).
type RolePermissionsResponse struct {
	Role        string               `json:"role"`
	Permissions map[string]AccessDTO `json:"permissions"`
	// Paths maps each page key to its web route, for menus and page guards.
	Paths map[string]string `json:"paths,omitempty"`
}

// PagePaths returns page key -> web route for every configurable page.
func PagePaths() map[string]string {
	out := make(map[string]string, len(domainPermission.Catalog))
	for _, p := range domainPermission.Catalog {
		out[p.Key] = p.Path
	}
	return out
}

func toAccessDTO(a domainPermission.Access) AccessDTO {
	return AccessDTO{View: a.View, Create: a.Create, Edit: a.Edit, Delete: a.Delete}
}

func NewRoleAccessDTO(ra usecasePermission.RoleAccess) map[string]AccessDTO {
	out := make(map[string]AccessDTO, len(ra))
	for k, v := range ra {
		out[k] = toAccessDTO(v)
	}
	return out
}

func (r UpdateRolePermissionsRequest) ToRoleAccess() usecasePermission.RoleAccess {
	out := make(usecasePermission.RoleAccess, len(r.Permissions))
	for k, v := range r.Permissions {
		out[k] = domainPermission.Access{View: v.View, Create: v.Create, Edit: v.Edit, Delete: v.Delete}
	}
	return out
}

func NewPermissionMatrixResponse(m *usecasePermission.Matrix) PermissionMatrixResponse {
	pages := make([]PermissionPageResponse, 0, len(m.Pages))
	for _, p := range m.Pages {
		actions := make([]string, 0, len(p.Actions))
		for _, a := range p.Actions {
			actions = append(actions, string(a))
		}
		pages = append(pages, PermissionPageResponse{Key: p.Key, Label: p.Label, Group: p.Group, Path: p.Path, Actions: actions})
	}
	perms := make(map[string]map[string]AccessDTO, len(m.Permissions))
	for role, ra := range m.Permissions {
		perms[role] = NewRoleAccessDTO(ra)
	}
	return PermissionMatrixResponse{Roles: m.Roles, Pages: pages, Permissions: perms}
}
