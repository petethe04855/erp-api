package permission

import (
	"context"
	"testing"

	domain "chawy-erp-api/internal/domain/permission"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	rows  map[string]domain.RolePermission // role|page -> row
	lists int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[string]domain.RolePermission{}} }

func (f *fakeRepo) ListAll(ctx context.Context) ([]domain.RolePermission, error) {
	f.lists++
	out := make([]domain.RolePermission, 0, len(f.rows))
	for _, r := range f.rows {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRepo) SaveRole(ctx context.Context, role string, rows []domain.RolePermission) error {
	for _, r := range rows {
		r.Role = role
		f.rows[role+"|"+r.PageKey] = r
	}
	return nil
}

func TestDefaultsMirrorThePreviousRoleRules(t *testing.T) {
	uc := NewUsecase(newFakeRepo())
	ctx := context.Background()

	cases := []struct {
		role, page string
		action     domain.Action
		want       bool
	}{
		{"warehouse", domain.PageSKU, domain.Create, true},
		{"accountant", domain.PageSKU, domain.Delete, true},
		{"sales", domain.PageSKU, domain.Delete, false},
		{"accountant", domain.PageFinanceExpenses, domain.Edit, true},
		{"sales", domain.PageFinanceExpenses, domain.View, false},
		{"live", domain.PageMyLive, domain.Create, true},
		{"live", domain.PageOrders, domain.View, false},
	}
	for _, c := range cases {
		got, err := uc.Can(ctx, c.role, c.page, c.action)
		require.NoError(t, err)
		assert.Equal(t, c.want, got, "%s %s %s", c.role, c.page, c.action)
	}
}

func TestOwnerAlwaysAllowedAndUnknownRoleDenied(t *testing.T) {
	uc := NewUsecase(newFakeRepo())
	ctx := context.Background()

	ok, err := uc.Can(ctx, "Owner", domain.PageFinanceExpenses, domain.Delete)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = uc.Can(ctx, "intern", domain.PageDashboard, domain.View)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestUpdateRoleOverridesDefaultsAndInvalidatesCache(t *testing.T) {
	repo := newFakeRepo()
	uc := NewUsecase(repo)
	ctx := context.Background()

	ok, _ := uc.Can(ctx, "sales", domain.PageFinanceExpenses, domain.Create)
	require.False(t, ok)

	ra, err := uc.UpdateRole(ctx, "sales", RoleAccess{
		domain.PageFinanceExpenses: {View: true, Create: true},
		domain.PageSKU:             {View: true}, // drop create/edit
	}, "owner@example.com")
	require.NoError(t, err)
	assert.True(t, ra[domain.PageFinanceExpenses].Create)

	ok, _ = uc.Can(ctx, "sales", domain.PageFinanceExpenses, domain.Create)
	assert.True(t, ok, "saved grant applies immediately")
	ok, _ = uc.Can(ctx, "sales", domain.PageSKU, domain.Edit)
	assert.False(t, ok, "saved revoke applies immediately")
	ok, _ = uc.Can(ctx, "sales", domain.PageCustomers, domain.Create)
	assert.True(t, ok, "pages not in the request keep their default")

	assert.Equal(t, "owner@example.com", repo.rows["sales|"+domain.PageFinanceExpenses].UpdatedBy)
}

func TestUpdateRoleDropsActionsThePageDoesNotHave(t *testing.T) {
	uc := NewUsecase(newFakeRepo())
	ra, err := uc.UpdateRole(context.Background(), "sales", RoleAccess{
		domain.PageDashboard: {View: true, Create: true, Edit: true, Delete: true},
	}, "")
	require.NoError(t, err)
	assert.Equal(t, domain.Access{View: true}, ra[domain.PageDashboard])
}

func TestUpdateRoleRejectsOwnerUnknownRoleAndUnknownPage(t *testing.T) {
	uc := NewUsecase(newFakeRepo())
	ctx := context.Background()
	in := RoleAccess{domain.PageSKU: {View: true}}

	codeOf := func(err error) string {
		var appErr *appErrors.AppError
		require.ErrorAs(t, err, &appErr)
		return appErr.Code
	}

	_, err := uc.UpdateRole(ctx, "owner", in, "")
	assert.Equal(t, "OWNER_NOT_EDITABLE", codeOf(err))

	_, err = uc.UpdateRole(ctx, "admin", in, "")
	assert.Equal(t, "INVALID_ROLE", codeOf(err))

	_, err = uc.UpdateRole(ctx, "sales", RoleAccess{"nope": {View: true}}, "")
	assert.Equal(t, "INVALID_PAGE", codeOf(err))

	_, err = uc.UpdateRole(ctx, "sales", RoleAccess{}, "")
	assert.Equal(t, "VALIDATION_ERROR", codeOf(err))
}

func TestChecksAreServedFromCache(t *testing.T) {
	repo := newFakeRepo()
	uc := NewUsecase(repo)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = uc.Can(ctx, "sales", domain.PageSKU, domain.View)
	}
	assert.Equal(t, 1, repo.lists)
}

func TestMatrixListsEveryRoleAndPage(t *testing.T) {
	m, err := NewUsecase(newFakeRepo()).GetMatrix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, domain.ConfigurableRoles, m.Roles)
	for _, role := range domain.ConfigurableRoles {
		assert.Len(t, m.Permissions[role], len(domain.Catalog))
	}
}

func TestCatalogKeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range domain.Catalog {
		assert.False(t, seen[p.Key], "duplicate page key %s", p.Key)
		seen[p.Key] = true
		assert.True(t, p.Supports(domain.View), "page %s must support view", p.Key)
	}
}
