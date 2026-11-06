# Clean Architecture Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `erp-api-v2` backend codebase to resolve architectural rule violations: decouple usecases and handlers from direct `*gorm.DB` access, implement missing `ReportRepository`, decouple accessories resolution and COGS journal posting, implement proper response DTOs for Order/Invoice/Purchasing, and normalize API routing.

**Architecture:** Enforce Clean Layered Architecture: Delivery (Handlers/DTOs) -> Usecase (Business Logic & Transactions via `database.TxManager`) -> Domain (Entities & Repository Contracts) -> Repository (Postgres Implementations). Handlers and Usecases must not directly execute SQL or maintain raw DB connections.

**Tech Stack:** Go 1.23+, Fiber v2, GORM, PostgreSQL, testify.

## Global Constraints
- Never read, inspect, or modify any `.env*` files.
- Keep all unit tests passing (`go test ./...`).
- Preserve existing API JSON schemas and backward compatibility for existing frontend clients.
- Use `pkg/response` for standard standardized HTTP responses (`OK`, `Created`, `List`, `BadRequest`, `NotFound`).

---

### Task 1: Create `ReportRepository` and Decouple `ReportUsecase` from `*gorm.DB`

**Files:**
- Create: `internal/domain/report/repository.go`
- Create: `internal/repository/postgres/report_repository.go`
- Modify: `internal/usecase/report/usecase.go`
- Modify: `cmd/api/main.go`
- Modify: `internal/usecase/report/usecase_test.go`

**Interfaces:**
- Consumes: `*gorm.DB` (only inside `internal/repository/postgres/report_repository.go`)
- Produces: `domainReport.Repository` with methods:
  - `GetDashboardSummary(ctx context.Context) (*domainReport.DashboardSummary, error)`
  - `GetRevenueReport(ctx context.Context, month string) (*domainReport.RevenueReport, error)`
  - `GetFinancialSummary(ctx context.Context, month string) (*domainReport.FinancialSummary, error)`
  - `GetInventoryValuation(ctx context.Context) (*domainReport.InventoryValuation, error)`

- [ ] **Step 1: Define `domainReport.Repository` contract**

Create `internal/domain/report/repository.go`:
```go
package report

import (
	"context"
)

type Repository interface {
	GetDashboardSummary(ctx context.Context) (*DashboardSummary, error)
	GetRevenueReport(ctx context.Context, month string) (*RevenueReport, error)
	GetFinancialSummary(ctx context.Context, month string) (*FinancialSummary, error)
	GetInventoryValuation(ctx context.Context) (*InventoryValuation, error)
}
```

- [ ] **Step 2: Implement `postgres.ReportRepository`**

Create `internal/repository/postgres/report_repository.go` implementing the database queries currently residing in `reportUsecase`.

- [ ] **Step 3: Refactor `reportUsecase` to depend on `domainReport.Repository`**

Update `internal/usecase/report/usecase.go` to accept `repo domainReport.Repository` instead of `db *gorm.DB`.

- [ ] **Step 4: Update `main.go` and `usecase_test.go`**

Wire `reportRepo := postgres.NewReportRepository(db)` and pass to `usecaseReport.NewReportUsecase(reportRepo)`.
In `usecase_test.go`, pass `postgres.NewReportRepository(db)` to `NewReportUsecase`.

- [ ] **Step 5: Run tests to verify**

Run: `go test ./internal/usecase/report/...`
Expected: PASS

---

### Task 2: Decouple SKU Accessories Query in `stock.DeductionResolver`

**Files:**
- Modify: `internal/domain/sku/entity.go`
- Modify: `internal/repository/postgres/sku_repository.go`
- Modify: `internal/usecase/stock/resolver.go`
- Modify: `internal/usecase/stock/usecase.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Consumes: `domainSKU.Repository`
- Produces: `FindAccessoriesBySKU(ctx context.Context, sku string) ([]SKUAccessory, error)` on `domainSKU.Repository`

- [ ] **Step 1: Add `FindAccessoriesBySKU` to `domainSKU.Repository`**

In `internal/domain/sku/entity.go`, add:
```go
FindAccessoriesBySKU(ctx context.Context, sku string) ([]SKUAccessory, error)
```

- [ ] **Step 2: Implement in `internal/repository/postgres/sku_repository.go`**

```go
func (r *skuRepository) FindAccessoriesBySKU(ctx context.Context, skuCode string) ([]domainSKU.SKUAccessory, error) {
	var accessories []domainSKU.SKUAccessory
	db := database.GetDBFromContext(ctx, r.db)
	err := db.Where("UPPER(sku) = ?", strings.ToUpper(strings.TrimSpace(skuCode))).Find(&accessories).Error
	return accessories, err
}
```

- [ ] **Step 3: Remove `*gorm.DB` from `internal/usecase/stock/resolver.go`**

Refactor `deductionResolver` to remove `db *gorm.DB` and call `r.skuRepo.FindAccessoriesBySKU(ctx, skuCode)`.

- [ ] **Step 4: Update resolver instantiation in `stock/usecase.go` and `cmd/api/main.go`**

Remove `db` argument from `NewDeductionResolver`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/usecase/stock/...`
Expected: PASS

---

### Task 3: Decouple COGS Journal Entry Creation in `OrderUsecase`

**Files:**
- Modify: `internal/usecase/order/usecase.go`
- Modify: `cmd/api/main.go`
- Modify: `internal/usecase/order/usecase_test.go`

**Interfaces:**
- Consumes: `domainFinance.Repository`
- Produces: Decoupled journal entry creation without `u.db`

- [ ] **Step 1: Update `orderUsecase` dependencies**

In `internal/usecase/order/usecase.go`:
- Remove `db *gorm.DB` from `orderUsecase` struct.
- Add `financeRepo domainFinance.Repository`.
- In `ShipOrder`, replace direct GORM calls (`dbTx := u.db.WithContext(txCtx)`) with `u.financeRepo.FindAccountByCode(txCtx, ...)` and `u.financeRepo.CreateJournalEntry(txCtx, ...)`.

- [ ] **Step 2: Update `cmd/api/main.go` and tests**

Pass `financeRepo` to `NewOrderUsecase(...)`.
Update test mock/setup in `internal/usecase/order/usecase_test.go`.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/usecase/order/...`
Expected: PASS

---

### Task 4: Decouple `InvoiceHandler` from Direct DB Access

**Files:**
- Modify: `internal/usecase/invoice/usecase.go`
- Modify: `internal/delivery/http/handler/invoice_handler.go`
- Modify: `cmd/api/main.go`

**Interfaces:**
- Consumes: `usecaseInvoice.Usecase`
- Produces: PDF generation data provider in Usecase, removing `db *gorm.DB` from `InvoiceHandler`.

- [ ] **Step 1: Add PDF data retrieval method to `usecaseInvoice.Usecase`**

In `internal/usecase/invoice/usecase.go`, add `GetInvoiceDetailForPDF(ctx context.Context, id uint) (*domainInvoice.Invoice, *domainOrder.Order, *domainCustomer.Customer, error)`.
Implement it using `invRepo`, `orderRepo`, and inject `customerRepo domainCustomer.Repository` into `invoiceUsecase`.

- [ ] **Step 2: Remove `db *gorm.DB` from `InvoiceHandler`**

In `internal/delivery/http/handler/invoice_handler.go`:
- Remove `db *gorm.DB` field and constructor parameter.
- Update `ExportPDF` to call `h.usecase.GetInvoiceDetailForPDF(c.Context(), uint(id))`.

- [ ] **Step 3: Update `cmd/api/main.go`**

Update `handler.NewInvoiceHandler(invoiceUsecase, settingsUsecase)`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/delivery/http/handler/... ./internal/usecase/invoice/...`
Expected: PASS

---

### Task 5: Implement Response DTOs for Order, Invoice, and Purchasing

**Files:**
- Modify: `internal/delivery/http/dto/order_dto.go`
- Modify: `internal/delivery/http/dto/invoice_dto.go`
- Modify: `internal/delivery/http/dto/purchasing_dto.go`
- Modify: `internal/delivery/http/handler/order_handler.go`
- Modify: `internal/delivery/http/handler/invoice_handler.go`
- Modify: `internal/delivery/http/handler/purchasing_handler.go`

**Interfaces:**
- Consumes: Domain entities (`domainOrder.Order`, `domainInvoice.Invoice`, etc.)
- Produces: Serialized Response DTOs via handler standard responses.

- [ ] **Step 1: Define Response DTOs**

Add `OrderResponse`, `ToOrderResponse`, `ToOrderResponses` in `order_dto.go`.
Add `InvoiceResponse`, `ToInvoiceResponse`, `ToInvoiceResponses` in `invoice_dto.go`.
Add `SupplierResponse`, `PurchaseOrderResponse`, and corresponding mappers in `purchasing_dto.go`.

- [ ] **Step 2: Update Handlers to return DTOs**

Update `order_handler.go`, `invoice_handler.go`, `purchasing_handler.go` to wrap entities in DTOs before passing to `response.OK` / `response.Created` / `response.List`.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/delivery/http/handler/...`
Expected: PASS

---

### Task 6: Clean Up API Route Aliases and Ensure `/api/v1` Standards

**Files:**
- Modify: `internal/delivery/http/route/route.go`

**Interfaces:**
- Consumes: Handlers & Middleware
- Produces: Consistent `/api/v1/...` route registration with clear documentation for any deprecated compatibility aliases.

- [ ] **Step 1: Organize route grouping and remove redundant root mappings**

Review `/api/inventory-formulas` alias and make sure all standard routes are mapped under `api := cfg.App.Group("/api/v1")`.
Add deprecation comment headers for legacy aliases.

- [ ] **Step 2: Run all tests in repository**

Run: `go test ./...`
Expected: All packages PASS
