package handler

import (
	"errors"
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	"chawy-erp-api/internal/delivery/http/middleware"
	domainAuth "chawy-erp-api/internal/domain/auth"
	usecaseAuth "chawy-erp-api/internal/usecase/auth"
	appErrors "chawy-erp-api/pkg/errors"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	usecase usecaseAuth.Usecase
}

func NewAuthHandler(usecase usecaseAuth.Usecase) *AuthHandler {
	return &AuthHandler{usecase: usecase}
}

func toUserResponse(u *domainAuth.User) dto.UserResponse {
	return dto.UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		Firstname:   u.Firstname,
		Lastname:    u.Lastname,
		Name:        u.Name,
		Role:        u.Role,
		IsActive:    u.IsActive,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "Email and password are required")
	}

	result, err := h.usecase.Register(c.Context(), usecaseAuth.RegisterInput{
		Email:     req.Email,
		Password:  req.Password,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Name:      req.Name,
		Role:      req.Role,
	})
	if err != nil {
		if errors.Is(err, appErrors.ErrUserAlreadyExists) {
			return response.Error(c, fiber.StatusConflict, "USER_ALREADY_EXISTS", err.Error())
		}
		return err
	}

	res := dto.AuthResponse{
		Token: result.Token,
		User:  toUserResponse(result.User),
	}

	return response.Created(c, res, "User registered successfully")
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "Email and password are required")
	}

	result, err := h.usecase.Login(c.Context(), usecaseAuth.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, appErrors.ErrAccountDisabled) {
			return response.Forbidden(c, err.Error(), "ACCOUNT_DISABLED")
		}
		if errors.Is(err, appErrors.ErrInvalidCredentials) {
			return response.Unauthorized(c, err.Error(), "INVALID_CREDENTIALS")
		}
		return err
	}

	res := dto.AuthResponse{
		Token: result.Token,
		User:  toUserResponse(result.User),
	}

	return response.OK(c, res, "Login successful")
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	user, err := h.usecase.GetProfile(c.Context(), userID)
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return response.NotFound(c, "User not found")
		}
		return err
	}

	return response.OK(c, toUserResponse(user))
}

// GetPermissionMatrix returns the RBAC Permission Matrix for frontend consumption (API-01)
func (h *AuthHandler) GetPermissionMatrix(c *fiber.Ctx) error {
	return response.OK(c, middleware.PermissionMatrix)
}

// ListUsers returns all users in the system (API-01)
func (h *AuthHandler) ListUsers(c *fiber.Ctx) error {
	users, err := h.usecase.ListUsers(c.Context())
	if err != nil {
		return err
	}

	list := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		list = append(list, toUserResponse(u))
	}

	return response.OK(c, list)
}

// GetUserByID returns user by ID (API-01)
func (h *AuthHandler) GetUserByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	user, err := h.usecase.GetUserByID(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return response.NotFound(c, "User not found")
		}
		return err
	}

	return response.OK(c, toUserResponse(user))
}

// CreateUser handles admin creation of users with assigned roles (API-01)
func (h *AuthHandler) CreateUser(c *fiber.Ctx) error {
	var req dto.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Email == "" || req.Password == "" {
		return response.BadRequest(c, "Email and password are required")
	}

	user, err := h.usecase.CreateUser(c.Context(), usecaseAuth.CreateUserInput{
		Email:     req.Email,
		Password:  req.Password,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Name:      req.Name,
		Role:      req.Role,
	})
	if err != nil {
		if errors.Is(err, appErrors.ErrUserAlreadyExists) {
			return response.Error(c, fiber.StatusConflict, "USER_ALREADY_EXISTS", err.Error())
		}
		if errors.Is(err, appErrors.ErrInvalidRole) {
			return response.BadRequest(c, err.Error(), "INVALID_ROLE")
		}
		if errors.Is(err, appErrors.ErrValidation) {
			return response.BadRequest(c, "Password must be at least 6 characters")
		}
		return err
	}

	return response.Created(c, toUserResponse(user), "User created successfully")
}

// UpdateUser handles admin updating user attributes (API-01)
func (h *AuthHandler) UpdateUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	currentUserID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	var req dto.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	user, err := h.usecase.UpdateUser(c.Context(), currentUserID, uint(id), usecaseAuth.UpdateUserInput{
		Email:     req.Email,
		Password:  req.Password,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Name:      req.Name,
		Role:      req.Role,
		IsActive:  req.IsActive,
	})
	if err != nil {
		if errors.Is(err, appErrors.ErrCannotModifySelf) {
			return response.BadRequest(c, err.Error(), "CANNOT_MODIFY_SELF")
		}
		if errors.Is(err, appErrors.ErrNotFound) {
			return response.NotFound(c, "User not found")
		}
		if errors.Is(err, appErrors.ErrUserAlreadyExists) {
			return response.Error(c, fiber.StatusConflict, "USER_ALREADY_EXISTS", err.Error())
		}
		if errors.Is(err, appErrors.ErrInvalidRole) {
			return response.BadRequest(c, err.Error(), "INVALID_ROLE")
		}
		if errors.Is(err, appErrors.ErrValidation) {
			return response.BadRequest(c, "Password must be at least 6 characters")
		}
		return err
	}

	return response.OK(c, toUserResponse(user), "User updated successfully")
}

// UpdateUserStatus toggles active/inactive state of a user (API-01)
func (h *AuthHandler) UpdateUserStatus(c *fiber.Ctx) error {
	targetID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	currentUserID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	var req dto.UpdateUserStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if err := h.usecase.UpdateUserStatus(c.Context(), currentUserID, uint(targetID), req.IsActive); err != nil {
		if errors.Is(err, appErrors.ErrCannotModifySelf) {
			return response.BadRequest(c, err.Error(), "CANNOT_MODIFY_SELF")
		}
		if errors.Is(err, appErrors.ErrNotFound) {
			return response.NotFound(c, "User not found")
		}
		return err
	}

	return response.OK(c, fiber.Map{"id": targetID, "isActive": req.IsActive}, "User status updated successfully")
}

// DeleteUser deletes a user account (API-01)
func (h *AuthHandler) DeleteUser(c *fiber.Ctx) error {
	targetID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	currentUserID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	if err := h.usecase.DeleteUser(c.Context(), currentUserID, uint(targetID)); err != nil {
		if errors.Is(err, appErrors.ErrCannotModifySelf) {
			return response.BadRequest(c, err.Error(), "CANNOT_MODIFY_SELF")
		}
		if errors.Is(err, appErrors.ErrNotFound) {
			return response.NotFound(c, "User not found")
		}
		return err
	}

	return response.OK(c, fiber.Map{"id": targetID}, "User deleted successfully")
}
