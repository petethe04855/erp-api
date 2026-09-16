package auth

import (
	"context"
	"strconv"
	"strings"
	"time"

	domainAuth "chawy-erp-api/internal/domain/auth"
	appErrors "chawy-erp-api/pkg/errors"
	"chawy-erp-api/pkg/jwt"

	"golang.org/x/crypto/bcrypt"
)

type RegisterInput struct {
	Email     string
	Password  string
	Firstname string
	Lastname  string
	Name      string
	Role      string
}

type LoginInput struct {
	Email    string
	Password string
}

type CreateUserInput struct {
	Email     string
	Password  string
	Firstname string
	Lastname  string
	Name      string
	Role      string
}

type UpdateUserInput struct {
	Email     *string
	Password  *string
	Firstname *string
	Lastname  *string
	Name      *string
	Role      *string
	IsActive  *bool
}

type AuthResult struct {
	Token string           `json:"token"`
	User  *domainAuth.User `json:"user"`
}

type Usecase interface {
	Register(ctx context.Context, input RegisterInput) (*AuthResult, error)
	Login(ctx context.Context, input LoginInput) (*AuthResult, error)
	GetProfile(ctx context.Context, userID uint) (*domainAuth.User, error)

	// Admin User Management (API-01)
	ListUsers(ctx context.Context) ([]*domainAuth.User, error)
	GetUserByID(ctx context.Context, id uint) (*domainAuth.User, error)
	CreateUser(ctx context.Context, input CreateUserInput) (*domainAuth.User, error)
	UpdateUser(ctx context.Context, currentUserID, id uint, input UpdateUserInput) (*domainAuth.User, error)
	UpdateUserStatus(ctx context.Context, currentUserID, targetID uint, isActive bool) error
	DeleteUser(ctx context.Context, currentUserID, targetID uint) error
}

type authUsecase struct {
	repo        domainAuth.Repository
	jwtSecret   string
	jwtExpHours int
}

func NewAuthUsecase(repo domainAuth.Repository, jwtSecret, jwtExpHours string) Usecase {
	exp, err := strconv.Atoi(jwtExpHours)
	if err != nil || exp <= 0 {
		exp = 24
	}
	return &authUsecase{
		repo:        repo,
		jwtSecret:   jwtSecret,
		jwtExpHours: exp,
	}
}

func isValidRole(role string) bool {
	switch role {
	case "owner", "sales", "warehouse", "accountant":
		return true
	default:
		return false
	}
}

func resolveName(name, firstname, lastname string) (string, string, string) {
	fn := strings.TrimSpace(firstname)
	ln := strings.TrimSpace(lastname)
	nm := strings.TrimSpace(name)

	if nm == "" {
		if fn != "" || ln != "" {
			nm = strings.TrimSpace(fn + " " + ln)
		}
	}
	if fn == "" && nm != "" {
		parts := strings.SplitN(nm, " ", 2)
		fn = parts[0]
		if len(parts) > 1 {
			ln = parts[1]
		}
	}
	return nm, fn, ln
}

func (u *authUsecase) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	exists, err := u.repo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, appErrors.ErrUserAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	name, fn, ln := resolveName(input.Name, input.Firstname, input.Lastname)

	user := &domainAuth.User{
		Email:     email,
		Password:  string(hashedPassword),
		Firstname: fn,
		Lastname:  ln,
		Name:      name,
		IsActive:  true,
	}

	// First-User-As-Owner bootstrap policy executed atomically (API-35)
	assignedRole, err := u.repo.CreateWithBootstrapRole(ctx, user)
	if err != nil {
		return nil, err
	}
	user.Role = assignedRole

	token, err := jwt.GenerateToken(user.ID, user.Email, user.Role, u.jwtSecret, u.jwtExpHours)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		Token: token,
		User:  user,
	}, nil
}

func (u *authUsecase) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	user, err := u.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, appErrors.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, appErrors.ErrAccountDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		return nil, appErrors.ErrInvalidCredentials
	}

	now := time.Now()
	user.LastLoginAt = &now
	_ = u.repo.Update(ctx, user)

	token, err := jwt.GenerateToken(user.ID, user.Email, user.Role, u.jwtSecret, u.jwtExpHours)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		Token: token,
		User:  user,
	}, nil
}

func (u *authUsecase) GetProfile(ctx context.Context, userID uint) (*domainAuth.User, error) {
	user, err := u.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, appErrors.ErrNotFound
	}
	if !user.IsActive {
		return nil, appErrors.ErrAccountDisabled
	}
	return user, nil
}

func (u *authUsecase) ListUsers(ctx context.Context) ([]*domainAuth.User, error) {
	return u.repo.FindAll(ctx)
}

func (u *authUsecase) GetUserByID(ctx context.Context, id uint) (*domainAuth.User, error) {
	user, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, appErrors.ErrNotFound
	}
	return user, nil
}

func (u *authUsecase) CreateUser(ctx context.Context, input CreateUserInput) (*domainAuth.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	exists, err := u.repo.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, appErrors.ErrUserAlreadyExists
	}

	role := strings.ToLower(strings.TrimSpace(input.Role))
	if role == "" {
		role = "sales"
	}
	if !isValidRole(role) {
		return nil, appErrors.ErrInvalidRole
	}

	if len(input.Password) < 6 {
		return nil, appErrors.ErrValidation
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	name, fn, ln := resolveName(input.Name, input.Firstname, input.Lastname)

	user := &domainAuth.User{
		Email:     email,
		Password:  string(hashedPassword),
		Firstname: fn,
		Lastname:  ln,
		Name:      name,
		Role:      role,
		IsActive:  true,
	}

	if err := u.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *authUsecase) UpdateUser(ctx context.Context, currentUserID, id uint, input UpdateUserInput) (*domainAuth.User, error) {
	user, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, appErrors.ErrNotFound
	}

	// API-34: Cannot disable self or demote self via general update
	if currentUserID == id {
		if input.IsActive != nil && !*input.IsActive {
			return nil, appErrors.ErrCannotModifySelf
		}
		if input.Role != nil && strings.ToLower(strings.TrimSpace(*input.Role)) != user.Role {
			return nil, appErrors.ErrCannotModifySelf
		}
	}

	if input.Email != nil {
		newEmail := strings.ToLower(strings.TrimSpace(*input.Email))
		if newEmail != user.Email {
			exists, err := u.repo.ExistsByEmail(ctx, newEmail)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, appErrors.ErrUserAlreadyExists
			}
			user.Email = newEmail
		}
	}

	if input.Role != nil {
		newRole := strings.ToLower(strings.TrimSpace(*input.Role))
		if !isValidRole(newRole) {
			return nil, appErrors.ErrInvalidRole
		}
		user.Role = newRole
	}

	if input.Password != nil && *input.Password != "" {
		if len(*input.Password) < 6 {
			return nil, appErrors.ErrValidation
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Password = string(hashedPassword)
	}

	if input.Firstname != nil {
		user.Firstname = strings.TrimSpace(*input.Firstname)
	}
	if input.Lastname != nil {
		user.Lastname = strings.TrimSpace(*input.Lastname)
	}
	if input.Name != nil {
		user.Name = strings.TrimSpace(*input.Name)
	}
	if user.Name == "" && (user.Firstname != "" || user.Lastname != "") {
		user.Name = strings.TrimSpace(user.Firstname + " " + user.Lastname)
	}

	if input.IsActive != nil {
		user.IsActive = *input.IsActive
	}

	if err := u.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *authUsecase) UpdateUserStatus(ctx context.Context, currentUserID, targetID uint, isActive bool) error {
	if currentUserID == targetID && !isActive {
		return appErrors.ErrCannotModifySelf
	}

	user, err := u.repo.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if user == nil {
		return appErrors.ErrNotFound
	}

	return u.repo.UpdateStatus(ctx, targetID, isActive)
}

func (u *authUsecase) DeleteUser(ctx context.Context, currentUserID, targetID uint) error {
	if currentUserID == targetID {
		return appErrors.ErrCannotModifySelf
	}

	user, err := u.repo.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if user == nil {
		return appErrors.ErrNotFound
	}

	return u.repo.Delete(ctx, targetID)
}
