package auth_test

import (
	"context"
	"testing"

	domainAuth "chawy-erp-api/internal/domain/auth"
	usecaseAuth "chawy-erp-api/internal/usecase/auth"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

type mockAuthRepo struct {
	users  map[uint]*domainAuth.User
	nextID uint
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{
		users:  make(map[uint]*domainAuth.User),
		nextID: 1,
	}
}

func (m *mockAuthRepo) FindByEmail(ctx context.Context, email string) (*domainAuth.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockAuthRepo) FindByID(ctx context.Context, id uint) (*domainAuth.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *mockAuthRepo) FindAll(ctx context.Context) ([]*domainAuth.User, error) {
	list := make([]*domainAuth.User, 0, len(m.users))
	for _, u := range m.users {
		list = append(list, u)
	}
	return list, nil
}

func (m *mockAuthRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

func (m *mockAuthRepo) Create(ctx context.Context, user *domainAuth.User) error {
	user.ID = m.nextID
	m.nextID++
	m.users[user.ID] = user
	return nil
}

func (m *mockAuthRepo) CreateWithBootstrapRole(ctx context.Context, user *domainAuth.User) (string, error) {
	var role string
	if len(m.users) == 0 {
		role = "owner"
	} else {
		role = "sales"
	}
	user.Role = role
	user.ID = m.nextID
	m.nextID++
	m.users[user.ID] = user
	return role, nil
}

func (m *mockAuthRepo) Update(ctx context.Context, user *domainAuth.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockAuthRepo) UpdateStatus(ctx context.Context, id uint, isActive bool) error {
	if u, ok := m.users[id]; ok {
		u.IsActive = isActive
		return nil
	}
	return appErrors.ErrNotFound
}

func (m *mockAuthRepo) Delete(ctx context.Context, id uint) error {
	delete(m.users, id)
	return nil
}

func (m *mockAuthRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	for _, u := range m.users {
		if u.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func TestRegister_FirstUserBecomesOwner(t *testing.T) {
	repo := newMockAuthRepo()
	uc := usecaseAuth.NewAuthUsecase(repo, "test-secret", "24")

	res, err := uc.Register(context.Background(), usecaseAuth.RegisterInput{
		Email:    "first@erp.local",
		Password: "password123",
		Name:     "Owner User",
		Role:     "anything", // Even if specified, first user MUST be owner
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "owner", res.User.Role)
	assert.True(t, res.User.IsActive)
}

func TestRegister_SubsequentUserDefaultsToSales(t *testing.T) {
	repo := newMockAuthRepo()
	uc := usecaseAuth.NewAuthUsecase(repo, "test-secret", "24")

	// First user
	_, err := uc.Register(context.Background(), usecaseAuth.RegisterInput{
		Email:    "first@erp.local",
		Password: "password123",
		Name:     "Owner User",
	})
	assert.NoError(t, err)

	// Second user attempting to claim owner/admin
	res2, err := uc.Register(context.Background(), usecaseAuth.RegisterInput{
		Email:    "second@erp.local",
		Password: "password123",
		Name:     "Second User",
		Role:     "owner", // Malicious attempt to claim owner
	})

	assert.NoError(t, err)
	assert.NotNil(t, res2)
	assert.Equal(t, "sales", res2.User.Role) // Must strictly be demoted to default sales
}

func TestLogin_DisabledAccountRejected(t *testing.T) {
	repo := newMockAuthRepo()
	uc := usecaseAuth.NewAuthUsecase(repo, "test-secret", "24")

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	disabledUser := &domainAuth.User{
		Email:    "disabled@erp.local",
		Password: string(hash),
		Role:     "sales",
		IsActive: false, // Disabled account
	}
	_ = repo.Create(context.Background(), disabledUser)

	_, err := uc.Login(context.Background(), usecaseAuth.LoginInput{
		Email:    "disabled@erp.local",
		Password: "password123",
	})

	assert.ErrorIs(t, err, appErrors.ErrAccountDisabled)
}

func TestUpdateUserStatus_PreventSelfDeactivation(t *testing.T) {
	repo := newMockAuthRepo()
	uc := usecaseAuth.NewAuthUsecase(repo, "test-secret", "24")

	owner := &domainAuth.User{
		Email:    "owner@erp.local",
		Role:     "owner",
		IsActive: true,
	}
	_ = repo.Create(context.Background(), owner)

	// Owner tries to disable their own account
	err := uc.UpdateUserStatus(context.Background(), owner.ID, owner.ID, false)
	assert.ErrorIs(t, err, appErrors.ErrCannotModifySelf)
}

func TestDeleteUser_PreventSelfDeletion(t *testing.T) {
	repo := newMockAuthRepo()
	uc := usecaseAuth.NewAuthUsecase(repo, "test-secret", "24")

	owner := &domainAuth.User{
		Email:    "owner@erp.local",
		Role:     "owner",
		IsActive: true,
	}
	_ = repo.Create(context.Background(), owner)

	// Owner tries to delete self
	err := uc.DeleteUser(context.Background(), owner.ID, owner.ID)
	assert.ErrorIs(t, err, appErrors.ErrCannotModifySelf)
}

func TestGetProfile_DisabledAccountRejected(t *testing.T) {
	repo := newMockAuthRepo()
	uc := usecaseAuth.NewAuthUsecase(repo, "test-secret", "24")

	disabledUser := &domainAuth.User{
		Email:    "disabled@erp.local",
		Role:     "sales",
		IsActive: false,
	}
	_ = repo.Create(context.Background(), disabledUser)

	_, err := uc.GetProfile(context.Background(), disabledUser.ID)
	assert.ErrorIs(t, err, appErrors.ErrAccountDisabled)
}

func TestUpdateUser_PreventSelfDeactivationOrRoleChange(t *testing.T) {
	repo := newMockAuthRepo()
	uc := usecaseAuth.NewAuthUsecase(repo, "test-secret", "24")

	owner := &domainAuth.User{
		Email:    "owner@erp.local",
		Role:     "owner",
		IsActive: true,
	}
	_ = repo.Create(context.Background(), owner)

	// 1. Owner tries to deactivate self via UpdateUser
	deact := false
	_, err := uc.UpdateUser(context.Background(), owner.ID, owner.ID, usecaseAuth.UpdateUserInput{
		IsActive: &deact,
	})
	assert.ErrorIs(t, err, appErrors.ErrCannotModifySelf)

	// 2. Owner tries to demote self to sales via UpdateUser
	demoteRole := "sales"
	_, err = uc.UpdateUser(context.Background(), owner.ID, owner.ID, usecaseAuth.UpdateUserInput{
		Role: &demoteRole,
	})
	assert.ErrorIs(t, err, appErrors.ErrCannotModifySelf)
}

