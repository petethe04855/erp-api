# Phase 1: Auth, User Management & RBAC Design Specification

**Date:** 2026-09-07  
**Status:** Approved  
**Target:** `erp-api-v2` (Clean Architecture)  
**Issues Addressed:** API-01 (User Management & RBAC), API-31 (Enforce RBAC & Register Role Policy)

---

## 1. Objectives & Scope

1. **Secure Registration (API-31)**:
   - Close vulnerability where callers specify `role` in `/auth/register`.
   - Implement **First-User-As-Owner** bootstrapping:
     - If user count == 0: assign role `"owner"`.
     - If user count > 0: ignore client-provided role and assign default `"sales"`.
2. **Account Status & Login Safety**:
   - Check `user.IsActive`. If `false`, return `403 Forbidden` (`ErrAccountDisabled`).
   - Track `last_login_at` on successful login.
3. **Full User Management (API-01)**:
   - Provide CRUD and activation endpoints for users (`/api/v1/users`), restricted strictly to `"owner"`.
   - Expose RBAC Permission Matrix at `/api/v1/rbac/permission-matrix`.
4. **Operation Route Protection (API-31)**:
   - Protect sensitive business endpoints (Stock adjustment, Order shipment/cancellation, PO approval/receipt, Invoice payment, SKU deletion) using `RequireRole` / `RequirePermission`.

---

## 2. Architecture & Data Structures

### 2.1 Domain Model (`internal/domain/auth/entity.go`)

```go
type User struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Email       string     `json:"email" gorm:"uniqueIndex;not null;size:255"`
	Password    string     `json:"-" gorm:"not null"`
	Firstname   string     `json:"firstname" gorm:"size:100"`
	Lastname    string     `json:"lastname" gorm:"size:100"`
	Name        string     `json:"name" gorm:"size:255"`
	Role        string     `json:"role" gorm:"size:50;default:'sales'"`
	IsActive    bool       `json:"isActive" gorm:"not null;default:true"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
```

### 2.2 Repository Interface (`internal/domain/auth/entity.go`)

```go
type Repository interface {
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uint) (*User, error)
	FindAll(ctx context.Context) ([]*User, error)
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	UpdateStatus(ctx context.Context, id uint, isActive bool) error
	Delete(ctx context.Context, id uint) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
```

### 2.3 Usecase Interface (`internal/usecase/auth/usecase.go`)

```go
type Usecase interface {
	Register(ctx context.Context, input RegisterInput) (*AuthResult, error)
	Login(ctx context.Context, input LoginInput) (*AuthResult, error)
	GetProfile(ctx context.Context, userID uint) (*domainAuth.User, error)

	// User Management (Admin / Owner)
	ListUsers(ctx context.Context) ([]*domainAuth.User, error)
	GetUserByID(ctx context.Context, id uint) (*domainAuth.User, error)
	CreateUser(ctx context.Context, input CreateUserInput) (*domainAuth.User, error)
	UpdateUser(ctx context.Context, id uint, input UpdateUserInput) (*domainAuth.User, error)
	UpdateUserStatus(ctx context.Context, currentUserID, targetID uint, isActive bool) error
	DeleteUser(ctx context.Context, currentUserID, targetID uint) error
}
```

### 2.4 RBAC Permission Matrix & Middleware (`internal/delivery/http/middleware`)

```go
var PermissionMatrix = map[string][]string{
	"View":    {"owner", "sales", "warehouse", "accountant"},
	"Create":  {"owner", "sales", "warehouse", "accountant"},
	"Edit":    {"owner", "sales", "warehouse", "accountant"},
	"Delete":  {"owner", "accountant"},
	"Approve": {"owner", "accountant", "warehouse"},
	"Post":    {"owner", "accountant"},
	"Cancel":  {"owner", "accountant", "warehouse"},
	"Reverse": {"owner", "accountant"},
	"Export":  {"owner", "sales", "warehouse", "accountant"},
}
```

---

## 3. Endpoints Contract

| Method | Path | Auth / Role | Description |
|---|---|---|---|
| POST | `/api/v1/auth/register` | Public | Register user (first user is owner, others are sales) |
| POST | `/api/v1/auth/login` | Public | Login with email & password |
| GET | `/api/v1/auth/me` | Protected | Current user profile |
| GET | `/api/v1/rbac/permission-matrix` | Protected | Return Permission Matrix JSON |
| GET | `/api/v1/users` | Protected: `owner` | List all users |
| GET | `/api/v1/users/:id` | Protected: `owner` | Get user by ID |
| POST | `/api/v1/users` | Protected: `owner` | Create user with specific role |
| PUT | `/api/v1/users/:id` | Protected: `owner` | Update user details & role |
| PUT | `/api/v1/users/:id/status` | Protected: `owner` | Toggle user active status (cannot disable self) |
| DELETE | `/api/v1/users/:id` | Protected: `owner` | Delete user (cannot delete self) |

---

## 4. Error Handling

- `ErrAccountDisabled`: HTTP 403 Forbidden (`ACCOUNT_DISABLED`)
- `ErrCannotModifySelf`: HTTP 400 Bad Request (`CANNOT_MODIFY_SELF`)
- `ErrInvalidRole`: HTTP 400 Bad Request (`INVALID_ROLE`)
- `ErrUserAlreadyExists`: HTTP 409 Conflict (`USER_ALREADY_EXISTS`)
- `ErrNotFound`: HTTP 404 Not Found (`USER_NOT_FOUND`)
