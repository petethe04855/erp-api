// Package permission holds the per-role page access policy that the owner
// edits on the "role permissions" page: for every page, whether a role may
// view it and create, edit or delete its data.
package permission

import (
	"context"
	"time"
)

// Action is one of the four rights a role can hold on a page.
type Action string

const (
	View   Action = "view"
	Create Action = "create"
	Edit   Action = "edit"
	Delete Action = "delete"
)

// AllActions lists the actions in display order.
var AllActions = []Action{View, Create, Edit, Delete}

// OwnerRole always holds every right; it is not stored and cannot be edited.
const OwnerRole = "owner"

// ConfigurableRoles are the roles the owner can grant rights to. They must
// match usecase/auth.isValidRole (minus owner) and the frontend UserRole type.
var ConfigurableRoles = []string{"sales", "warehouse", "accountant", "live"}

// IsConfigurableRole reports whether role can be edited on the settings page.
func IsConfigurableRole(role string) bool {
	for _, r := range ConfigurableRoles {
		if r == role {
			return true
		}
	}
	return false
}

// Access is the set of rights one role holds on one page.
type Access struct {
	View   bool `json:"view"`
	Create bool `json:"create"`
	Edit   bool `json:"edit"`
	Delete bool `json:"delete"`
}

// Allows reports whether the access grants action.
func (a Access) Allows(action Action) bool {
	switch action {
	case View:
		return a.View
	case Create:
		return a.Create
	case Edit:
		return a.Edit
	case Delete:
		return a.Delete
	}
	return false
}

// Limit clears every right the page does not support.
func (a Access) Limit(page Page) Access {
	return Access{
		View:   a.View && page.Supports(View),
		Create: a.Create && page.Supports(Create),
		Edit:   a.Edit && page.Supports(Edit),
		Delete: a.Delete && page.Supports(Delete),
	}
}

// FullAccess grants every action the page supports (used for the owner).
func FullAccess(page Page) Access {
	return Access{View: true, Create: true, Edit: true, Delete: true}.Limit(page)
}

// RolePermission is one stored row of the matrix. A missing row means the
// role still has the page's default access (see Page.Defaults).
type RolePermission struct {
	ID        uint      `gorm:"primaryKey"`
	Role      string    `gorm:"size:50;not null;uniqueIndex:idx_role_permissions_role_page"`
	PageKey   string    `gorm:"size:100;not null;uniqueIndex:idx_role_permissions_role_page"`
	CanView   bool      `gorm:"not null;default:false"`
	CanCreate bool      `gorm:"not null;default:false"`
	CanEdit   bool      `gorm:"not null;default:false"`
	CanDelete bool      `gorm:"not null;default:false"`
	UpdatedBy string    `gorm:"size:255"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (RolePermission) TableName() string { return "role_permissions" }

// Access converts the stored row to an Access value.
func (r RolePermission) Access() Access {
	return Access{View: r.CanView, Create: r.CanCreate, Edit: r.CanEdit, Delete: r.CanDelete}
}

// Repository persists the matrix rows.
type Repository interface {
	ListAll(ctx context.Context) ([]RolePermission, error)
	// SaveRole upserts every row of one role in a single transaction.
	SaveRole(ctx context.Context, role string, rows []RolePermission) error
}
