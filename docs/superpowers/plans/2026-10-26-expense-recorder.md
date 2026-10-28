# Expense Recorder (Created By) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to specify/edit the expense recorder in the Expense Modal (auto-defaulting to the current authenticated user), display the recorder in the Expense Table, and support search by recorder in the backend.

**Architecture:** Extend backend DTOs and Handler/Usecase in `erp-api-v2` to accept `created_by` (with fallback to JWT user email), add search capability in `finance_repo.go`, and update `ExpenseModal` and `ExpenseTable` in `erp-web-v2` with modern UI and Zustand `useAuthStore` integration.

**Tech Stack:** Go (Fiber, GORM, PostgreSQL), Next.js (App Router), TypeScript, Tailwind CSS, Lucide Icons, Zustand (`useAuthStore`).

## Global Constraints
- Strictly adhere to Clean Architecture: Handlers handle HTTP, Usecase handles business rules, Repositories handle database persistence.
- Do NOT read or modify any `.env` files.
- Preserve existing double-entry GL journal posting and P&L calculations.

---

### Task 1: Backend DTOs, Handler, Usecase & Repository Search (`erp-api-v2`)

**Files:**
- Modify: `erp-api-v2/internal/delivery/http/dto/finance_dto.go`
- Modify: `erp-api-v2/internal/usecase/finance/usecase.go`
- Modify: `erp-api-v2/internal/usecase/finance/posting.go`
- Modify: `erp-api-v2/internal/delivery/http/handler/finance_handler.go`
- Modify: `erp-api-v2/internal/repository/postgres/finance_repo.go`

**Interfaces:**
- Produces: `CreatedBy string` in `CreateExpenseRequest` and `UpdateExpenseRequest`.
- Produces: `CreatedBy string` in `UpdateExpenseInput`.
- Produces: `created_by ILIKE ?` search condition in `finance_repo.go`.

- [ ] **Step 1: Update DTOs in `internal/delivery/http/dto/finance_dto.go`**
Add `CreatedBy string` `json:"created_by"` to `CreateExpenseRequest` and `UpdateExpenseRequest`:
```go
type CreateExpenseRequest struct {
	Date        string  `json:"date"`
	Category    string  `json:"category"`
	Channel     string  `json:"channel"`
	Amount      float64 `json:"amount"`
	ProductName string  `json:"product_name"`
	PurchaseURL string  `json:"purchase_url"`
	SlipURL     string  `json:"slip_url"`
	Vendor      string  `json:"vendor"`
	InvoiceRef  string  `json:"invoice_ref"`
	Description string  `json:"description"`
	CreatedBy   string  `json:"created_by"`
}

type UpdateExpenseRequest struct {
	Date        string  `json:"date"`
	Category    string  `json:"category"`
	Channel     string  `json:"channel"`
	Amount      float64 `json:"amount"`
	ProductName string  `json:"product_name"`
	PurchaseURL string  `json:"purchase_url"`
	SlipURL     string  `json:"slip_url"`
	Vendor      string  `json:"vendor"`
	InvoiceRef  string  `json:"invoice_ref"`
	Description string  `json:"description"`
	CreatedBy   string  `json:"created_by"`
}
```

- [ ] **Step 2: Update Usecase input in `internal/usecase/finance/usecase.go` and `posting.go`**
In `internal/usecase/finance/usecase.go`:
Add `CreatedBy string` `json:"created_by"` to `UpdateExpenseInput`:
```go
type UpdateExpenseInput struct {
	Date        string
	Category    domainFinance.ExpenseCategory
	Channel     domainFinance.ExpenseChannel
	Amount      float64
	ProductName string
	PurchaseURL string
	SlipURL     string
	Vendor      string
	InvoiceRef  string
	Description string
	CreatedBy   string
}
```

In `internal/usecase/finance/posting.go`:
In `UpdateExpense`, assign `exp.CreatedBy = in.CreatedBy` if `strings.TrimSpace(in.CreatedBy) != ""`:
```go
	if strings.TrimSpace(in.CreatedBy) != "" {
		exp.CreatedBy = strings.TrimSpace(in.CreatedBy)
	}
```

- [ ] **Step 3: Update Handler in `internal/delivery/http/handler/finance_handler.go`**
In `CreateExpense`:
```go
	createdBy := strings.TrimSpace(req.CreatedBy)
	if createdBy == "" {
		username, _ := c.Locals("email").(string)
		if username == "" {
			username = "system"
		}
		createdBy = username
	}
```
Pass `CreatedBy: createdBy` into `usecaseFinance.CreateExpenseInput`.

In `UpdateExpense`:
Pass `CreatedBy: strings.TrimSpace(req.CreatedBy)` into `usecaseFinance.UpdateExpenseInput`.

- [ ] **Step 4: Update Repository Search in `internal/repository/postgres/finance_repo.go`**
In `ListExpenses`, append `OR created_by ILIKE ?` to the search condition:
```go
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		q = q.Where("code ILIKE ? OR vendor ILIKE ? OR invoice_ref ILIKE ? OR description ILIKE ? OR product_name ILIKE ? OR created_by ILIKE ?", searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	}
```

- [ ] **Step 5: Run tests in `erp-api-v2`**
Run: `go test ./internal/usecase/finance/... -v`
Expected: PASS

---

### Task 2: Frontend Types & ExpenseModal Component Updates (`erp-web-v2`)

**Files:**
- Modify: `erp-web-v2/features/finance/types/finance.ts:70-84`
- Modify: `erp-web-v2/features/finance/components/ExpenseModal.tsx`

**Interfaces:**
- Produces: `created_by?: string;` in `CreateExpenseDTO` and `UpdateExpenseDTO`.
- Produces: "ผู้ลงบันทึก" form input with auto-fill from `useAuthStore`.

- [ ] **Step 1: Update `features/finance/types/finance.ts`**
Add `created_by?: string;` to `CreateExpenseDTO`:
```typescript
export interface CreateExpenseDTO {
  date: string;
  category?: ExpenseCategory;
  channel?: ExpenseChannel;
  amount: number;
  product_name?: string;
  purchase_url?: string;
  slip_url?: string;
  vendor?: string;
  invoice_ref?: string;
  description?: string;
  created_by?: string;
}
```

- [ ] **Step 2: Update `features/finance/components/ExpenseModal.tsx`**
- Import `User` icon from `lucide-react`.
- Import `useAuthStore` from `@/stores/authStore`.
- Read `currentUser = useAuthStore((s) => s.user)`.
- In `useEffect`:
  - When `initialData` exists (edit mode): `created_by: initialData.created_by || ""`.
  - When creating new: `created_by: currentUser?.name || currentUser?.email || ""`.
- Add an input for "ผู้ลงบันทึก" in the form (placed neatly next to Date and Amount, or directly under Date & Amount):
```tsx
<div>
  <label className="flex items-center gap-1.5 text-xs font-semibold text-neutral-700 mb-1">
    <User size={14} className="text-neutral-500" />
    ผู้ลงบันทึก
  </label>
  <input
    type="text"
    placeholder="เช่น สมชาย ใจดี หรือชื่อผู้ลงค่าใช้จ่าย"
    value={formData.created_by || ""}
    onChange={(e) =>
      setFormData({ ...formData, created_by: e.target.value })
    }
    className="w-full rounded-lg border border-neutral-300 px-3 py-2 text-sm focus:border-neutral-900 focus:outline-none focus:ring-1 focus:ring-neutral-900"
  />
</div>
```

---

### Task 3: Frontend ExpenseTable Updates & Full Verification (`erp-web-v2`)

**Files:**
- Modify: `erp-web-v2/features/finance/components/ExpenseTable.tsx`

**Interfaces:**
- Produces: "ผู้บันทึก" column in `ExpenseTable`.

- [ ] **Step 1: Update `features/finance/components/ExpenseTable.tsx`**
- Import `User as UserIcon` from `lucide-react`.
- In `<thead>`:
  Add `<th className="px-4 py-3">ผู้บันทึก</th>` (e.g. before "การจัดการ" or after "ช่องทาง").
- In `<tbody>`:
  Add cell:
```tsx
<td className="px-4 py-3 whitespace-nowrap">
  <div className="flex items-center gap-1.5 text-xs text-neutral-700">
    <UserIcon size={13} className="text-neutral-400 flex-shrink-0" />
    <span className="truncate max-w-[120px]" title={exp.created_by || "-"}>
      {exp.created_by || "-"}
    </span>
  </div>
</td>
```

- [ ] **Step 2: Typecheck & Build validation in `erp-web-v2`**
Run: `npm run type-check` (or `npx tsc --noEmit`) in `erp-web-v2`.
Expected: 0 errors.

- [ ] **Step 3: Verification**
Verify end-to-end functionality:
1. Open `/finance/expenses`.
2. Click "บันทึกค่าใช้จ่าย", verify "ผู้ลงบันทึก" is auto-filled.
3. Test table display has "ผู้บันทึก" column.
4. Test search query works with recorder name.
