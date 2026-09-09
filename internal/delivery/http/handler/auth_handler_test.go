package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"chawy-erp-api/internal/delivery/http/dto"
	"chawy-erp-api/internal/delivery/http/handler"
	"chawy-erp-api/internal/delivery/http/route"
	domainAuth "chawy-erp-api/internal/domain/auth"
	usecaseAuth "chawy-erp-api/internal/usecase/auth"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

type mockRepo struct {
	users  map[uint]*domainAuth.User
	nextID uint
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:  make(map[uint]*domainAuth.User),
		nextID: 1,
	}
}

func (m *mockRepo) FindByEmail(ctx context.Context, email string) (*domainAuth.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) FindByID(ctx context.Context, id uint) (*domainAuth.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *mockRepo) FindAll(ctx context.Context) ([]*domainAuth.User, error) {
	list := make([]*domainAuth.User, 0, len(m.users))
	for _, u := range m.users {
		list = append(list, u)
	}
	return list, nil
}

func (m *mockRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

func (m *mockRepo) Create(ctx context.Context, user *domainAuth.User) error {
	user.ID = m.nextID
	m.nextID++
	m.users[user.ID] = user
	return nil
}

func (m *mockRepo) CreateWithBootstrapRole(ctx context.Context, user *domainAuth.User) (string, error) {
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

func (m *mockRepo) Update(ctx context.Context, user *domainAuth.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockRepo) UpdateStatus(ctx context.Context, id uint, isActive bool) error {
	if u, ok := m.users[id]; ok {
		u.IsActive = isActive
		return nil
	}
	return appErrors.ErrNotFound
}

func (m *mockRepo) Delete(ctx context.Context, id uint) error {
	delete(m.users, id)
	return nil
}

func (m *mockRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	for _, u := range m.users {
		if u.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func setupTestApp(repo domainAuth.Repository, secret string) *fiber.App {
	app := fiber.New()
	uc := usecaseAuth.NewAuthUsecase(repo, secret, "24")
	authH := handler.NewAuthHandler(uc)

	route.RegisterRoutes(route.Config{
		App:         app,
		AuthHandler: authH,
		JWTSecret:   secret,
	})
	return app
}

func TestAuthFlowAndUserManagement(t *testing.T) {
	repo := newMockRepo()
	jwtSecret := "test-secret-key-12345"
	app := setupTestApp(repo, jwtSecret)

	// 1. Register first user -> must be owner
	regBody, _ := json.Marshal(dto.RegisterRequest{
		Email:    "owner@erp.local",
		Password: "password123",
		Name:     "Owner One",
		Role:     "anything",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var regRes struct {
		Success bool `json:"success"`
		Data    struct {
			Token string           `json:"token"`
			User  dto.UserResponse `json:"user"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&regRes)
	assert.Equal(t, "owner", regRes.Data.User.Role)
	ownerToken := regRes.Data.Token

	// 2. Register second user -> must be sales even if asking for owner
	regBody2, _ := json.Marshal(dto.RegisterRequest{
		Email:    "sales@erp.local",
		Password: "password123",
		Name:     "Sales One",
		Role:     "owner",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBody2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp2.StatusCode)

	var regRes2 struct {
		Data struct {
			Token string           `json:"token"`
			User  dto.UserResponse `json:"user"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&regRes2)
	assert.Equal(t, "sales", regRes2.Data.User.Role)
	salesToken := regRes2.Data.Token

	// 3. Get Permission Matrix
	reqMatrix := httptest.NewRequest(http.MethodGet, "/api/v1/rbac/permission-matrix", nil)
	reqMatrix.Header.Set("Authorization", "Bearer "+ownerToken)
	respMatrix, err := app.Test(reqMatrix)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, respMatrix.StatusCode)

	// 4. Sales user tries to list users -> 403 Forbidden
	reqListForbidden := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	reqListForbidden.Header.Set("Authorization", "Bearer "+salesToken)
	respForbidden, err := app.Test(reqListForbidden)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, respForbidden.StatusCode)

	// 5. Owner lists users -> 200 OK
	reqListOK := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	reqListOK.Header.Set("Authorization", "Bearer "+ownerToken)
	respOK, err := app.Test(reqListOK)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, respOK.StatusCode)

	// 6. Owner tries to deactivate self -> 400 Bad Request (CANNOT_MODIFY_SELF)
	statusBody, _ := json.Marshal(dto.UpdateUserStatusRequest{IsActive: false})
	reqSelfDeact := httptest.NewRequest(http.MethodPut, "/api/v1/users/1/status", bytes.NewReader(statusBody))
	reqSelfDeact.Header.Set("Authorization", "Bearer "+ownerToken)
	reqSelfDeact.Header.Set("Content-Type", "application/json")
	respSelfDeact, err := app.Test(reqSelfDeact)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, respSelfDeact.StatusCode)

	// 7. Owner deactivates sales user -> 200 OK
	reqSalesDeact := httptest.NewRequest(http.MethodPut, "/api/v1/users/2/status", bytes.NewReader(statusBody))
	reqSalesDeact.Header.Set("Authorization", "Bearer "+ownerToken)
	reqSalesDeact.Header.Set("Content-Type", "application/json")
	respSalesDeact, err := app.Test(reqSalesDeact)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, respSalesDeact.StatusCode)

	// 8. Deactivated user attempts login -> 403 Forbidden (ACCOUNT_DISABLED)
	loginBody, _ := json.Marshal(dto.LoginRequest{
		Email:    "sales@erp.local",
		Password: "password123",
	})
	reqLoginDisabled := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	reqLoginDisabled.Header.Set("Content-Type", "application/json")
	respLoginDisabled, err := app.Test(reqLoginDisabled)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, respLoginDisabled.StatusCode)

	// 9. Owner tries to deactivate self via PUT /users/1 (API-34) -> 400 Bad Request
	deactUpdateBody, _ := json.Marshal(dto.UpdateUserRequest{
		IsActive: func() *bool { b := false; return &b }(),
	})
	reqPutSelfDeact := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", bytes.NewReader(deactUpdateBody))
	reqPutSelfDeact.Header.Set("Authorization", "Bearer "+ownerToken)
	reqPutSelfDeact.Header.Set("Content-Type", "application/json")
	respPutSelfDeact, err := app.Test(reqPutSelfDeact)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, respPutSelfDeact.StatusCode)
}
