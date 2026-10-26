# Design Spec: Expense Recorder (Created By) Specification & Display

**Date**: 2026-10-26  
**Status**: In Review  

---

## 1. Overview & Objectives

In the ERP system, expenses are managed in `erp-web-v2` on the Finance > Expenses page (`/finance/expenses`) backed by `erp-api-v2`. Currently, the database table and domain entity `Expense` already contain a `created_by` field, but:
1. The frontend `ExpenseModal` does not expose an input to specify or review who is recording the expense.
2. The `ExpenseTable` does not display who recorded each expense item.
3. The API DTOs (`CreateExpenseRequest`, `UpdateExpenseRequest`) do not allow passing an explicit `created_by` override, falling back strictly to the JWT user's email.

### Core Objectives
1. **Auto-fill & Customizable in Modal (`ExpenseModal.tsx`)**:
   - When creating a new expense, automatically default the "ผู้ลงบันทึก" (Expense Recorder) field to the currently logged-in user (`user.name` or `user.email`).
   - Allow users to edit the recorder name freely (e.g. if entering on behalf of a colleague or specifying a department/nickname).
   - When editing an existing expense, preserve and display the existing `created_by` value, allowing updates if needed.
2. **Display in Table (`ExpenseTable.tsx`)**:
   - Add a dedicated column **"ผู้บันทึก"** (Recorded By) in the table showing who logged the expense, with a clean user icon and badge.
3. **Backend Support (`erp-api-v2`)**:
   - Accept `created_by` in `CreateExpenseRequest` and `UpdateExpenseRequest` DTOs.
   - If provided in request body, use that value; otherwise fallback to JWT token's email/username.
   - Support searching by `created_by` in the search filter query (`finance_repo.go`).

---

## 2. Architecture & Data Flow

```text
User opens Expense Modal
  │
  ├─ 1. useAuthStore provides currently authenticated user (user.name || user.email)
  │     └─ Form field "ผู้ลงบันทึก" auto-populates with user's name/email
  │     └─ User can modify if recording on behalf of someone else
  │
  ├─ 2. User Submits Expense Payload (JSON):
  │     POST /api/v1/finance/expenses
  │     {
  │       "date": "2026-10-26",
  │       "amount": 1250.00,
  │       "product_name": "กล่องพัสดุ เบอร์ 2A",
  │       "created_by": "สมชาย ใจดี",
  │       ...
  │     }
  │
  ├─ 3. Backend Handler & Usecase:
  │     - Handler reads req.CreatedBy; if empty, falls back to c.Locals("email")
  │     - Saves to database (`expenses.created_by`)
  │     - Passes `created_by` to auto-posted Journal Entry audit trail
  │
  └─ 4. Table view displays "ผู้บันทึก" column with badge/icon and enables search by recorder
```

---

## 3. Backend Changes (`erp-api-v2`)

### 3.1 DTOs (`internal/delivery/http/dto/finance_dto.go`)
Add `CreatedBy string` `json:"created_by"` to:
- `CreateExpenseRequest`
- `UpdateExpenseRequest`

### 3.2 Handler (`internal/delivery/http/handler/finance_handler.go`)
- In `CreateExpense`:
  ```go
  createdBy := strings.TrimSpace(req.CreatedBy)
  if createdBy == "" {
      createdBy, _ = c.Locals("email").(string)
      if createdBy == "" {
          createdBy = "system"
      }
  }
  ```
- In `UpdateExpense`:
  - Pass `CreatedBy: strings.TrimSpace(req.CreatedBy)` to `usecase.UpdateExpenseInput`.

### 3.3 Usecase (`internal/usecase/finance/usecase.go` & `posting.go`)
- Add `CreatedBy string` `json:"created_by"` to `UpdateExpenseInput`.
- In `UpdateExpense`:
  - If `in.CreatedBy != ""`, update `exp.CreatedBy = in.CreatedBy`.

### 3.4 Repository Search (`internal/repository/postgres/finance_repo.go`)
- Update `ListExpenses`:
  ```go
  if filter.Search != "" {
      searchPattern := "%" + filter.Search + "%"
      q = q.Where(
          "code ILIKE ? OR vendor ILIKE ? OR invoice_ref ILIKE ? OR description ILIKE ? OR product_name ILIKE ? OR created_by ILIKE ?",
          searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern,
      )
  }
  ```

---

## 4. Frontend Changes (`erp-web-v2`)

### 4.1 Types (`features/finance/types/finance.ts`)
Add `created_by?: string;` to:
- `CreateExpenseDTO`
- `UpdateExpenseDTO`

*(Note: `Expense` interface already contains `created_by: string;`).*

### 4.2 Modal Form (`features/finance/components/ExpenseModal.tsx`)
- Import `useAuthStore` from `@/stores/authStore` and `User as UserIcon` from `lucide-react`.
- When opening modal:
  - If `initialData` exists (editing): set `formData.created_by = initialData.created_by || ""`.
  - If creating new: set `formData.created_by = currentUser?.name || currentUser?.email || ""`.
- Layout:
  - Add "ผู้ลงบันทึก / ผู้ทำรายการ" field in the form grid (e.g. alongside `Date` and `Amount` in a 3-column grid or 2-column layout).
  - Include placeholder "ระบุชื่อผู้ลงบันทึกค่าใช้จ่าย".

### 4.3 Table Display (`features/finance/components/ExpenseTable.tsx`)
- In table header:
  - Add `<th className="px-4 py-3">ผู้บันทึก</th>` (placed before "การจัดการ" or alongside "หมวดหมู่ / ช่องทาง").
- In table body row:
  - Add `<td className="px-4 py-3 whitespace-nowrap">`:
    ```tsx
    <div className="flex items-center gap-1.5 text-xs text-neutral-700">
      <UserIcon size={13} className="text-neutral-400 flex-shrink-0" />
      <span className="truncate max-w-[120px]" title={exp.created_by || "-"}>
        {exp.created_by || "-"}
      </span>
    </div>
    ```

---

## 5. Verification & Testing

1. **Unit / Build Tests**:
   - Run `go test ./...` in `erp-api-v2` to verify compilation and existing finance tests pass.
   - Run `npm run build` or `npm run type-check` in `erp-web-v2` to verify TypeScript types pass.
2. **Manual / Functional Verification**:
   - Log in as a user, open `/finance/expenses`, click "บันทึกค่าใช้จ่าย".
   - Verify the "ผู้ลงบันทึก" field defaults to the current user's name/email.
   - Change the name to a test name (e.g. "ทดสอบ ผู้บันทึก") and save.
   - Check the table row: verify the column "ผู้บันทึก" displays "ทดสอบ ผู้บันทึก".
   - Test search filter: type the recorder's name in the search box to verify filtering by recorder works.
   - Edit the expense: verify the existing recorder's name loads properly and can be updated.
