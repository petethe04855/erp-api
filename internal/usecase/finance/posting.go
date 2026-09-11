package finance

import (
	"context"
	"fmt"
	"math"
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

	// Generate code JE-YYYY-XXXX
	code := fmt.Sprintf("JE-%s-%04d", time.Now().Format("2006"), time.Now().UnixNano()%10000)

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
		return nil, appErrors.NewAppError("INVALID_CATEGORY", "Expense category is required", 400)
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
		Vendor:      in.Vendor,
		InvoiceRef:  in.InvoiceRef,
		Description: in.Description,
		CreatedBy:   in.CreatedBy,
	}

	run := func(txCtx context.Context) error {
		if err := u.repo.CreateExpense(txCtx, exp); err != nil {
			return err
		}

		// Auto-post journal for this expense
		_, err := u.PostJournal(txCtx, PostingRequest{
			Date:        exp.Date,
			SourceType:  "expense",
			SourceID:    exp.ID,
			SourceRef:   exp.Code,
			Description: fmt.Sprintf("[%s] %s - %s", exp.Category, exp.Vendor, exp.Description),
			CreatedBy:   exp.CreatedBy,
			Lines: []PostingLineInput{
				{AccountCode: AccountOperatingExpense, Debit: exp.Amount, Channel: string(exp.Channel)},
				{AccountCode: AccountCash, Credit: exp.Amount, Channel: string(exp.Channel)},
			},
		})
		return err
	}

	if u.txMgr != nil {
		if err := u.txMgr.Transaction(ctx, run); err != nil {
			return nil, err
		}
	} else {
		if err := run(ctx); err != nil {
			return nil, err
		}
	}

	return exp, nil
}

func (u *financeUsecase) UpdateExpense(ctx context.Context, id uint, in UpdateExpenseInput) (*domainFinance.Expense, error) {
	exp, err := u.repo.GetExpenseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if exp == nil {
		return nil, appErrors.ErrNotFound
	}

	if in.Amount <= 0 {
		return nil, appErrors.NewAppError("INVALID_AMOUNT", "Expense amount must be greater than zero", 400)
	}
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
	exp.Vendor = in.Vendor
	exp.InvoiceRef = in.InvoiceRef
	exp.Description = in.Description

	if err := u.repo.UpdateExpense(ctx, exp); err != nil {
		return nil, err
	}

	return exp, nil
}

func (u *financeUsecase) DeleteExpense(ctx context.Context, id uint) error {
	exp, err := u.repo.GetExpenseByID(ctx, id)
	if err != nil {
		return err
	}
	if exp == nil {
		return appErrors.ErrNotFound
	}
	return u.repo.DeleteExpense(ctx, id)
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

func (u *financeUsecase) ListExpenses(ctx context.Context, filter domainFinance.ExpenseFilter) ([]domainFinance.Expense, int64, error) {
	return u.repo.ListExpenses(ctx, filter)
}
