// Package permission applies the owner-edited page access matrix: it merges
// stored rows over the catalog defaults, validates edits and answers the
// per-request "may this role do X on page Y" check used by the middleware.
package permission

import (
	"context"
	"strings"
	"sync"
	"time"

	domain "chawy-erp-api/internal/domain/permission"
	appErrors "chawy-erp-api/pkg/errors"
)

// RoleAccess maps page key to the role's access on it.
type RoleAccess map[string]domain.Access

// Matrix is everything the settings page needs.
type Matrix struct {
	Roles       []string
	Pages       []domain.Page
	Permissions map[string]RoleAccess // role -> page -> access
}

type Usecase interface {
	GetMatrix(ctx context.Context) (*Matrix, error)
	// UpdateRole replaces the access of one role on the given pages. Pages
	// not in the input keep their current access.
	UpdateRole(ctx context.Context, role string, in RoleAccess, updatedBy string) (RoleAccess, error)
	// GetRoleAccess returns the effective access of role on every page.
	GetRoleAccess(ctx context.Context, role string) (RoleAccess, error)
	// Can reports whether role may perform action on page.
	Can(ctx context.Context, role, page string, action domain.Action) (bool, error)
}

// cacheTTL bounds how stale another API instance can be after a save; the
// instance that saved invalidates its own cache immediately.
const cacheTTL = 30 * time.Second

type usecase struct {
	repo domain.Repository
	now  func() time.Time

	mu       sync.RWMutex
	cache    map[string]RoleAccess
	loadedAt time.Time
}

func NewUsecase(repo domain.Repository) Usecase {
	return &usecase{repo: repo, now: time.Now}
}

func normalizeRole(role string) string { return strings.ToLower(strings.TrimSpace(role)) }

// load returns the effective matrix of every configurable role, from cache
// while it is fresh.
func (u *usecase) load(ctx context.Context) (map[string]RoleAccess, error) {
	u.mu.RLock()
	if u.cache != nil && u.now().Sub(u.loadedAt) < cacheTTL {
		c := u.cache
		u.mu.RUnlock()
		return c, nil
	}
	u.mu.RUnlock()

	rows, err := u.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	stored := map[string]map[string]domain.Access{}
	for _, r := range rows {
		role := normalizeRole(r.Role)
		if stored[role] == nil {
			stored[role] = map[string]domain.Access{}
		}
		stored[role][r.PageKey] = r.Access()
	}

	out := make(map[string]RoleAccess, len(domain.ConfigurableRoles))
	for _, role := range domain.ConfigurableRoles {
		ra := make(RoleAccess, len(domain.Catalog))
		for _, page := range domain.Catalog {
			if a, ok := stored[role][page.Key]; ok {
				ra[page.Key] = a.Limit(page)
			} else {
				ra[page.Key] = page.DefaultFor(role)
			}
		}
		out[role] = ra
	}

	u.mu.Lock()
	u.cache = out
	u.loadedAt = u.now()
	u.mu.Unlock()
	return out, nil
}

func (u *usecase) invalidate() {
	u.mu.Lock()
	u.cache = nil
	u.mu.Unlock()
}

func ownerAccess() RoleAccess {
	ra := make(RoleAccess, len(domain.Catalog))
	for _, page := range domain.Catalog {
		ra[page.Key] = domain.FullAccess(page)
	}
	return ra
}

func copyAccess(src RoleAccess) RoleAccess {
	dst := make(RoleAccess, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (u *usecase) GetMatrix(ctx context.Context) (*Matrix, error) {
	all, err := u.load(ctx)
	if err != nil {
		return nil, err
	}
	perms := make(map[string]RoleAccess, len(all))
	for role, ra := range all {
		perms[role] = copyAccess(ra)
	}
	return &Matrix{
		Roles:       append([]string(nil), domain.ConfigurableRoles...),
		Pages:       domain.Catalog,
		Permissions: perms,
	}, nil
}

func (u *usecase) GetRoleAccess(ctx context.Context, role string) (RoleAccess, error) {
	role = normalizeRole(role)
	if role == domain.OwnerRole {
		return ownerAccess(), nil
	}
	all, err := u.load(ctx)
	if err != nil {
		return nil, err
	}
	ra, ok := all[role]
	if !ok {
		// Unknown role: no rights anywhere.
		return RoleAccess{}, nil
	}
	return copyAccess(ra), nil
}

func (u *usecase) Can(ctx context.Context, role, page string, action domain.Action) (bool, error) {
	role = normalizeRole(role)
	if role == domain.OwnerRole {
		return true, nil
	}
	all, err := u.load(ctx)
	if err != nil {
		return false, err
	}
	return all[role][page].Allows(action), nil
}

func (u *usecase) UpdateRole(ctx context.Context, role string, in RoleAccess, updatedBy string) (RoleAccess, error) {
	role = normalizeRole(role)
	if role == domain.OwnerRole {
		return nil, appErrors.NewAppError("OWNER_NOT_EDITABLE", "สิทธิ์ของ Owner แก้ไขไม่ได้ (Owner มีสิทธิ์ทุกหน้าเสมอ)", 400)
	}
	if !domain.IsConfigurableRole(role) {
		return nil, appErrors.NewAppError("INVALID_ROLE", "ไม่พบ role นี้ในระบบ", 400)
	}
	if len(in) == 0 {
		return nil, appErrors.NewAppError("VALIDATION_ERROR", "ต้องระบุสิทธิ์อย่างน้อย 1 หน้า", 400)
	}

	rows := make([]domain.RolePermission, 0, len(in))
	for key, access := range in {
		page, ok := domain.FindPage(key)
		if !ok {
			return nil, appErrors.NewAppError("INVALID_PAGE", "ไม่พบหน้า "+key+" ในระบบ", 400)
		}
		a := access.Limit(page)
		rows = append(rows, domain.RolePermission{
			Role:      role,
			PageKey:   page.Key,
			CanView:   a.View,
			CanCreate: a.Create,
			CanEdit:   a.Edit,
			CanDelete: a.Delete,
			UpdatedBy: updatedBy,
		})
	}

	if err := u.repo.SaveRole(ctx, role, rows); err != nil {
		return nil, err
	}
	u.invalidate()
	return u.GetRoleAccess(ctx, role)
}
