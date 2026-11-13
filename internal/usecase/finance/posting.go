package finance

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	domainFinance "chawy-erp-api/internal/domain/finance"
	appErrors "chawy-erp-api/pkg/errors"
)

const (
	AccountCash             = "1100"
	AccountBank             = "1110"
	AccountAR               = "1200"
	AccountInventory        = "1300"
	AccountAP               = "2000"
	AccountVATOutput        = "2100"
	AccountRevenue          = "4000"
	AccountCOGS             = "5000"
	AccountSalesReturn      = "5100"
	AccountOperatingExpense = "6000"
)

func validatePostingLines(lines []PostingLineInput) error {
	if len(lines) < 2 {
		return appErrors.NewAppError("INVALID_JOURNAL", "Journal requires at least two lines", 400)
	}

	totalDebit := 0.0
	totalCredit := 0.0

	for _, line := range lines {
		if line.AccountCode == "" {
			return appErrors.NewAppError("INVALID_JOURNAL_LINE", "Account code is required on each line", 400)
		}
		if line.Debit < 0 || line.Credit < 0 {
			return appErrors.NewAppError("INVALID_JOURNAL_LINE", "Debit and credit must be non-negative", 400)
		}
		if (line.Debit > 0 && line.Credit > 0) || (line.Debit == 0 && line.Credit == 0) {
			return appErrors.NewAppError("INVALID_JOURNAL_LINE", "Journal line must contain either debit or credit", 400)
		}
		totalDebit += line.Debit
		totalCredit += line.Credit
	}

	if totalDebit <= 0 || math.Abs(totalDebit-totalCredit) > 0.005 {
		return appErrors.NewAppError("JOURNAL_UNBALANCED", fmt.Sprintf("Journal is not balanced: debit %.2f credit %.2f", totalDebit, totalCredit), 400)
	}

	return nil
}

func (u *financeUsecase) PostJournal(ctx context.Context, req PostingRequest) (*domainFinance.JournalEntry, error) {
	if req.SourceType == "" || req.SourceID == 0 {
		return nil, appErrors.NewAppError("INVALID_SOURCE", "Journal source type and ID are required", 400)
	}

	if err := validatePostingLines(req.Lines); err != nil {
		return nil, err
	}

	// Check idempotency: if entry for this source already exists, return it
	existing, err := u.repo.FindJournalEntryBySource(ctx, req.SourceType, req.SourceID)
	if err == nil && existing != nil {
		return existing, nil
	}

	dateStr := req.Date
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	// Generate code JE-YYYY/MM/DD-XXXX
	code := fmt.Sprintf("JE-%s-%04d", time.Now().Format("2006/01/02"), time.Now().UnixNano()%10000)

	entry := &domainFinance.JournalEntry{
		Code:        code,
		Date:        dateStr,
		SourceType:  req.SourceType,
		SourceID:    req.SourceID,
		SourceRef:   req.SourceRef,
		Description: req.Description,
		Status:      domainFinance.JournalStatusPosted,
		CreatedBy:   req.CreatedBy,
		PostedAt:    time.Now().UTC(),
		Lines:       make([]domainFinance.JournalLine, 0, len(req.Lines)),
	}

	for _, l := range req.Lines {
		account, err := u.repo.GetAccountByCode(ctx, l.AccountCode)
		if err != nil || account == nil {
			return nil, appErrors.NewAppError("ACCOUNT_NOT_FOUND", fmt.Sprintf("Active account with code %s not found", l.AccountCode), 400)
		}

		line := domainFinance.JournalLine{
			AccountID:   account.ID,
			AccountCode: account.Code,
			AccountName: account.Name,
			Debit:       l.Debit,
			Credit:      l.Credit,
			SKU:         l.SKU,
			Lot:         l.Lot,
			Channel:     l.Channel,
		}
		entry.Lines = append(entry.Lines, line)
	}

	if err := u.repo.CreateJournalEntry(ctx, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

func (u *financeUsecase) ListJournalEntries(ctx context.Context, filter domainFinance.JournalFilter) ([]domainFinance.JournalEntry, int64, error) {
	return u.repo.FindJournalEntries(ctx, filter)
}

func (u *financeUsecase) GetJournalEntryByID(ctx context.Context, id uint) (*domainFinance.JournalEntry, error) {
	entry, err := u.repo.FindJournalEntryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, appErrors.ErrNotFound
	}
	return entry, nil
}

// Expenses CRUD + Auto-Posting
func (u *financeUsecase) CreateExpense(ctx context.Context, in CreateExpenseInput) (*domainFinance.Expense, error) {
	if in.Amount <= 0 {
		return nil, appErrors.NewAppError("INVALID_AMOUNT", "Expense amount must be greater than zero", 400)
	}
	if in.Category == "" {
		in.Category = domainFinance.ExpenseCategorySGA
	}
	if in.Channel == "" {
		in.Channel = domainFinance.ExpenseChannelGeneral
	}
	dateStr := in.Date
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	code := fmt.Sprintf("EXP-%s-%04d", time.Now().Format("2006"), time.Now().UnixNano()%10000)

	exp := &domainFinance.Expense{
		Code:        code,
		Date:        dateStr,
		Category:    in.Category,
		Channel:     in.Channel,
		Amount:      in.Amount,
		ProductName: in.ProductName,
		PurchaseURL: in.PurchaseURL,
		SlipURL:     in.SlipURL,
		Vendor:      in.Vendor,
		InvoiceRef:  in.InvoiceRef,
		Description: in.Description,
		CreatedBy:   in.CreatedBy,
		Status:      domainFinance.ExpenseStatusActive,
	}

	run := func(txCtx context.Context) error {
		if err := u.repo.CreateExpense(txCtx, exp); err != nil {
			return err
		}
		_, err := u.PostJournal(txCtx, expensePostingRequest(exp, "expense", exp.ID, exp.CreatedBy))
		return err
	}

	if err := u.inTx(ctx, run); err != nil {
		return nil, err
	}
	return exp, nil
}

// Journal source types used for the expense lifecycle. Every entry carries
// SourceRef = expense code, so the full history of an expense can be found
// with FindJournalEntriesByRef. (source_type, source_id) is unique, so each
// type keys off a different ID:
//   - expense:          source_id = expense ID (first posting)
//   - expense_reversal: source_id = ID of the journal entry being reversed
//   - expense_revision: source_id = ID of the reversal it follows (re-post after edit)
const (
	sourceExpense         = "expense"
	sourceExpenseReversal = "expense_reversal"
	sourceExpenseRevision = "expense_revision"
)

// truncateRunes caps s at max characters without splitting a multi-byte (Thai) rune.
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func (u *financeUsecase) inTx(ctx context.Context, fn func(context.Context) error) error {
	if u.txMgr != nil {
		return u.txMgr.Transaction(ctx, fn)
	}
	return fn(ctx)
}

func expensePostingRequest(exp *domainFinance.Expense, sourceType string, sourceID uint, by string) PostingRequest {
	subject := exp.Vendor
	if subject == "" {
		subject = exp.ProductName
	} else if exp.ProductName != "" {
		subject = fmt.Sprintf("%s (%s)", exp.Vendor, exp.ProductName)
	}
	return PostingRequest{
		Date:        exp.Date,
		SourceType:  sourceType,
		SourceID:    sourceID,
		SourceRef:   exp.Code,
		Description: fmt.Sprintf("[%s] %s - %s", exp.Category, subject, exp.Description),
		CreatedBy:   by,
		Lines: []PostingLineInput{
			{AccountCode: AccountOperatingExpense, Debit: exp.Amount, Channel: string(exp.Channel)},
			{AccountCode: AccountCash, Credit: exp.Amount, Channel: string(exp.Channel)},
		},
	}
}

// activeExpenseJournal returns the expense's journal entry that currently
// counts in the books (posted, not itself a reversal, and not yet reversed),
// or nil when the expense has none (e.g. legacy rows created before posting).
func (u *financeUsecase) activeExpenseJournal(ctx context.Context, exp *domainFinance.Expense) (*domainFinance.JournalEntry, error) {
	entries, err := u.repo.FindJournalEntriesByRef(ctx, exp.Code)
	if err != nil {
		return nil, err
	}
	reversed := make(map[uint]bool)
	for _, e := range entries {
		if e.ReversalOfID != nil {
			reversed[*e.ReversalOfID] = true
		}
	}
	var active *domainFinance.JournalEntry
	for i := range entries {
		e := &entries[i]
		if e.SourceType != sourceExpense && e.SourceType != sourceExpenseRevision {
			continue
		}
		if e.ReversalOfID != nil || reversed[e.ID] || e.Status != domainFinance.JournalStatusPosted {
			continue
		}
		active = e // latest wins (entries are ordered by id)
	}
	return active, nil
}

// reverseJournal posts a mirror entry (debits and credits swapped) for the
// given entry, dated on the original date so period reports net to zero.
// The original entry is left untouched, which keeps the audit trail intact.
func (u *financeUsecase) reverseJournal(ctx context.Context, orig *domainFinance.JournalEntry, by, reason string) (*domainFinance.JournalEntry, error) {
	if existing, err := u.repo.FindJournalEntryBySource(ctx, sourceExpenseReversal, orig.ID); err == nil && existing != nil {
		return nil, appErrors.NewAppError("JOURNAL_ALREADY_REVERSED", fmt.Sprintf("Journal %s has already been reversed", orig.Code), 409)
	}

	origID := orig.ID
	desc := fmt.Sprintf("กลับรายการ %s", orig.Code)
	if reason != "" {
		desc = fmt.Sprintf("%s: %s", desc, reason)
	}
	desc = truncateRunes(desc, 255)

	rev := &domainFinance.JournalEntry{
		Code:         orig.Code + "-R",
		Date:         orig.Date,
		SourceType:   sourceExpenseReversal,
		SourceID:     orig.ID,
		SourceRef:    orig.SourceRef,
		Description:  desc,
		Status:       domainFinance.JournalStatusPosted,
		ReversalOfID: &origID,
		CreatedBy:    by,
		PostedAt:     time.Now().UTC(),
		Lines:        make([]domainFinance.JournalLine, 0, len(orig.Lines)),
	}
	for _, l := range orig.Lines {
		rev.Lines = append(rev.Lines, domainFinance.JournalLine{
			AccountID:   l.AccountID,
			AccountCode: l.AccountCode,
			AccountName: l.AccountName,
			Debit:       l.Credit,
			Credit:      l.Debit,
			SKU:         l.SKU,
			Lot:         l.Lot,
			Channel:     l.Channel,
		})
	}
	if err := u.repo.CreateJournalEntry(ctx, rev); err != nil {
		return nil, err
	}
	return rev, nil
}

func (u *financeUsecase) UpdateExpense(ctx context.Context, id uint, in UpdateExpenseInput) (*domainFinance.Expense, error) {
	exp, err := u.repo.GetExpenseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if exp == nil {
		return nil, appErrors.ErrNotFound
	}
	if exp.Status == domainFinance.ExpenseStatusVoided {
		return nil, appErrors.NewAppError("EXPENSE_VOIDED", "Voided expenses cannot be edited", 409)
	}

	if in.Amount <= 0 {
		return nil, appErrors.NewAppError("INVALID_AMOUNT", "Expense amount must be greater than zero", 400)
	}

	// Only these fields change the posted journal lines.
	oldAmount, oldDate, oldChannel := exp.Amount, exp.Date, exp.Channel

	if in.Category != "" {
		exp.Category = in.Category
	}
	if in.Channel != "" {
		exp.Channel = in.Channel
	}
	if in.Date != "" {
		exp.Date = in.Date
	}
	exp.Amount = in.Amount
	exp.ProductName = in.ProductName
	exp.PurchaseURL = in.PurchaseURL
	exp.SlipURL = in.SlipURL
	exp.Vendor = in.Vendor
	exp.InvoiceRef = in.InvoiceRef
	exp.Description = in.Description
	if strings.TrimSpace(in.CreatedBy) != "" {
		exp.CreatedBy = strings.TrimSpace(in.CreatedBy)
	}

	actor := strings.TrimSpace(in.UpdatedBy)
	if actor == "" {
		actor = exp.CreatedBy
	}
	needsRepost := math.Abs(oldAmount-exp.Amount) > 0.005 || oldDate != exp.Date || oldChannel != exp.Channel

	run := func(txCtx context.Context) error {
		if err := u.repo.UpdateExpense(txCtx, exp); err != nil {
			return err
		}
		if !needsRepost {
			return nil
		}

		active, err := u.activeExpenseJournal(txCtx, exp)
		if err != nil {
			return err
		}
		if active == nil {
			// Legacy expense that was never posted: post it now.
			if existing, _ := u.repo.FindJournalEntryBySource(txCtx, sourceExpense, exp.ID); existing == nil {
				_, err := u.PostJournal(txCtx, expensePostingRequest(exp, sourceExpense, exp.ID, actor))
				return err
			}
			return appErrors.NewAppError("JOURNAL_STATE_INVALID", fmt.Sprintf("Cannot find the active journal entry for expense %s", exp.Code), 409)
		}

		rev, err := u.reverseJournal(txCtx, active, actor, "แก้ไขค่าใช้จ่าย")
		if err != nil {
			return err
		}
		_, err = u.PostJournal(txCtx, expensePostingRequest(exp, sourceExpenseRevision, rev.ID, actor))
		return err
	}

	if err := u.inTx(ctx, run); err != nil {
		return nil, err
	}
	return exp, nil
}

func (u *financeUsecase) VoidExpense(ctx context.Context, id uint, actor, reason string) (*domainFinance.Expense, error) {
	exp, err := u.repo.GetExpenseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if exp == nil {
		return nil, appErrors.ErrNotFound
	}
	if exp.Status == domainFinance.ExpenseStatusVoided {
		return nil, appErrors.NewAppError("EXPENSE_ALREADY_VOIDED", "Expense has already been voided", 409)
	}

	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system"
	}
	reason = truncateRunes(strings.TrimSpace(reason), 255)

	run := func(txCtx context.Context) error {
		active, err := u.activeExpenseJournal(txCtx, exp)
		if err != nil {
			return err
		}
		if active != nil {
			revReason := "ยกเลิกค่าใช้จ่าย"
			if reason != "" {
				revReason = revReason + " - " + reason
			}
			if _, err := u.reverseJournal(txCtx, active, actor, revReason); err != nil {
				return err
			}
		}

		now := time.Now().UTC()
		exp.Status = domainFinance.ExpenseStatusVoided
		exp.VoidedAt = &now
		exp.VoidedBy = actor
		exp.VoidReason = reason
		return u.repo.UpdateExpense(txCtx, exp)
	}

	if err := u.inTx(ctx, run); err != nil {
		return nil, err
	}
	return exp, nil
}

func (u *financeUsecase) GetExpenseByID(ctx context.Context, id uint) (*domainFinance.Expense, error) {
	exp, err := u.repo.GetExpenseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if exp == nil {
		return nil, appErrors.ErrNotFound
	}
	return exp, nil
}

// GetExpenseByCode looks up the source expense of a journal entry (its source_ref).
func (u *financeUsecase) GetExpenseByCode(ctx context.Context, code string) (*domainFinance.Expense, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, appErrors.ErrNotFound
	}
	exp, err := u.repo.GetExpenseByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if exp == nil {
		return nil, appErrors.ErrNotFound
	}
	return exp, nil
}

func (u *financeUsecase) ListExpenses(ctx context.Context, filter domainFinance.ExpenseFilter) ([]domainFinance.Expense, int64, error) {
	return u.repo.ListExpenses(ctx, filter)
}
