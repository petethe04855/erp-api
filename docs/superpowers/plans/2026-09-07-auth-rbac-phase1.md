# Phase 1: Auth, User Management & RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement full user management and enforce RBAC policies (API-01, API-31) in `erp-api-v2`, including first-user bootstrap, role protection, and permission matrix.

**Architecture:** Clean Layered Architecture (Domain -> Repository -> Usecase -> Delivery -> Route) with PostgreSQL and Fiber HTTP framework.

**Tech Stack:** Go 1.22+, Fiber v2, GORM, PostgreSQL, JWT (golang-jwt/v5), bcrypt.

## Global Constraints

- Go API is the single source of truth for business logic and authorization.
- DTOs are strictly separated from Domain / Persistence models.
- All endpoints follow `/api/v1/...` versioning.
- Standard response envelope format: `{"success": true, "data": ..., "message": ...}`.
- Never read or modify `.env` files.

---

### Task 1: Domain Entities & App Errors

**Files:**
- Modify: `internal/domain/auth/entity.go`
- Modify: `pkg/errors/errors.go`

**Interfaces:**
- Consumes: standard library `context`, `time`
- Produces: `domainAuth.User`, `domainAuth.Repository`, `errors.ErrAccountDisabled`, `errors.ErrCannotModifySelf`, `errors.ErrInvalidRole`

- [ ] **Step 1: Update domain model and repository interface in `internal/domain/auth/entity.go`**
  - Add `Firstname`, `Lastname`, `IsActive`, `LastLoginAt` to `User` struct.
  - Expand `Repository` with `FindAll`, `Count`, `Update`, `UpdateStatus`, `Delete`.
- [ ] **Step 2: Add specific domain errors in `pkg/errors/errors.go`**
  - Add `ErrAccountDisabled = New(http.StatusForbidden, "ACCOUNT_DISABLED", "This account has been disabled")`
  - Add `ErrCannotModifySelf = New(http.StatusBadRequest, "CANNOT_MODIFY_SELF", "Cannot disable or delete your own account")`
  - Add `ErrInvalidRole = New(http.StatusBadRequest, "INVALID_ROLE", "Invalid user role")`
- [ ] **Step 3: Run `go build ./...` to verify domain changes compile**

---

### Task 2: Postgres Repository Implementation

**Files:**
- Modify: `internal/repository/postgres/auth_repository.go`
- Create: `internal/repository/postgres/auth_repository_test.go`

**Interfaces:**
- Consumes: `domainAuth.Repository`, `gorm.DB`
- Produces: complete SQL implementation of `Count`, `FindAll`, `Update`, `UpdateStatus`, `Delete`

- [ ] **Step 1: Implement `Count`, `FindAll`, `Update`, `UpdateStatus`, `Delete` in `auth_repository.go`**
- [ ] **Step 2: Write unit/mock test for repository methods in `auth_repository_test.go`**
- [ ] **Step 3: Run `go test ./internal/repository/postgres/...` to verify**

---

### Task 3: Usecase Logic (First-User Bootstrap, Active Check & User Management)

**Files:**
- Modify: `internal/usecase/auth/usecase.go`
- Create: `internal/usecase/auth/usecase_test.go`

**Interfaces:**
- Consumes: `domainAuth.Repository`, `pkg/jwt`, `pkg/errors`
- Produces: `Usecase` interface with `Register`, `Login`, `GetProfile`, `ListUsers`, `GetUserByID`, `CreateUser`, `UpdateUser`, `UpdateUserStatus`, `DeleteUser`

- [ ] **Step 1: Write failing unit tests in `usecase_test.go` for:**
  - First user registration gets role `"owner"`.
  - Subsequent user registration gets role `"sales"`, ignoring input role.
  - Inactive user login returns `ErrAccountDisabled`.
  - Prevent user from disabling or deleting self.
- [ ] **Step 2: Implement usecase methods in `usecase.go`**
- [ ] **Step 3: Run tests: `go test -v ./internal/usecase/auth/...` to verify they pass**

---

### Task 4: Delivery Layer (DTOs, RBAC Middleware & HTTP Handlers)

**Files:**
- Modify: `internal/delivery/http/dto/auth.go`
- Modify: `internal/delivery/http/middleware/auth_middleware.go`
- Modify: `internal/delivery/http/handler/auth_handler.go`
- Create: `internal/delivery/http/middleware/auth_middleware_test.go`

**Interfaces:**
- Consumes: `fiber.Ctx`, `usecaseAuth.Usecase`
- Produces: `AuthHandler.GetPermissionMatrix`, `AuthHandler.ListUsers`, `AuthHandler.GetUserByID`, `AuthHandler.CreateUser`, `AuthHandler.UpdateUser`, `AuthHandler.UpdateUserStatus`, `AuthHandler.DeleteUser`, `RequirePermission`

- [ ] **Step 1: Define DTOs in `dto/auth.go`**
  - `CreateUserRequest`, `UpdateUserRequest`, `UpdateUserStatusRequest`, `UserDetailResponse`
- [ ] **Step 2: Update `auth_middleware.go`**
  - Add `PermissionMatrix` mapping actions (`View`, `Create`, `Edit`, `Delete`, `Approve`, `Cancel`, `Post`, `Reverse`, `Export`).
  - Implement `RequirePermission(permission string) fiber.Handler`.
  - Add unit test in `auth_middleware_test.go`.
- [ ] **Step 3: Implement handler methods in `auth_handler.go`**
- [ ] **Step 4: Run `go test ./internal/delivery/http/...` to verify**

---

### Task 5: Route Registration & RBAC Enforcement

**Files:**
- Modify: `internal/delivery/http/route/route.go`
- Create: `internal/delivery/http/handler/auth_handler_test.go`

**Interfaces:**
- Consumes: `fiber.App`, handlers, middleware
- Produces: Protected `/api/v1/users` routes and role-guarded operational routes

- [ ] **Step 1: Register User Management and Permission Matrix routes in `route.go`**
  - `GET /api/v1/rbac/permission-matrix`
  - `GET /api/v1/users` (RequireRole: "owner")
  - `GET /api/v1/users/:id` (RequireRole: "owner")
  - `POST /api/v1/users` (RequireRole: "owner")
  - `PUT /api/v1/users/:id` (RequireRole: "owner")
  - `PUT /api/v1/users/:id/status` (RequireRole: "owner")
  - `DELETE /api/v1/users/:id` (RequireRole: "owner")
- [ ] **Step 2: Apply RBAC guards to operational routes**
  - `stocks.Post("/adjust", middleware.RequireRole("owner", "warehouse"), cfg.StockHandler.Adjust)`
  - `orders.Post("/:id/ship", middleware.RequireRole("owner", "warehouse", "sales"), cfg.OrderHandler.Ship)`
  - `orders.Post("/:id/cancel", middleware.RequireRole("owner", "sales"), cfg.OrderHandler.Cancel)`
  - `pos.Post("/:id/approve", middleware.RequireRole("owner", "accountant"), cfg.PurchasingHandler.ApprovePO)`
  - `invoices.Post("/:id/pay", middleware.RequireRole("owner", "accountant"), cfg.InvoiceHandler.MarkAsPaid)`
- [ ] **Step 3: Test route access with authorized and unauthorized roles**

---

### Task 6: End-to-End Verification

- [ ] **Step 1: Run `go build ./...` across entire `erp-api-v2`**
- [ ] **Step 2: Run all unit & integration tests `go test -v ./...`**
- [ ] **Step 3: Verify no regression in existing endpoints**
