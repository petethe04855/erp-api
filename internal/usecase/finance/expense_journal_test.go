package finance

import (
	"context"
	"testing"

	domainFinance "chawy-erp-api/internal/domain/finance"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func postedExpenseJournal(id uint, amount float64) domainFinance.JournalEntry {
	return domainFinance.JournalEntry{
		ID:         id,
		Code:       "JE-2026/11/01-0001",
		Date:       "2026-11-01",
		SourceType: sourceExpense,
		SourceID:   7,
		SourceRef:  "EXP-2026-0001",
		Status:     domainFinance.JournalStatusPosted,
		Lines: []domainFinance.JournalLine{
			{AccountID: 6, AccountCode: AccountOperatingExpense, AccountName: "Operating Expense", Debit: amount, Channel: "ทั่วไป"},
			{AccountID: 1, AccountCode: AccountCash, AccountName: "Cash", Credit: amount, Channel: "ทั่วไป"},
		},
	}
}

func activeExpense(amount float64) *domainFinance.Expense {
	return &domainFinance.Expense{
		ID: 7, Code: "EXP-2026-0001", Date: "2026-11-01", Amount: amount,
		Category: domainFinance.ExpenseCategorySGA, Channel: domainFinance.ExpenseChannelGeneral,
		CreatedBy: "staff@chawy.test", Status: domainFinance.ExpenseStatusActive,
	}
}

// netByAccount sums debit-credit per account across the given entries.
func netByAccount(entries ...*domainFinance.JournalEntry) map[string]float64 {
	out := map[string]float64{}
	for _, e := range entries {
		for _, l := range e.Lines {
			out[l.AccountCode] += l.Debit - l.Credit
		}
	}
	return out
}

func TestUpdateExpense_AmountChange_ReversesAndReposts(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	orig := postedExpenseJournal(100, 500)
	repo.On("GetExpenseByID", ctx, uint(7)).Return(activeExpense(500), nil)
	repo.On("UpdateExpense", ctx, mock.Anything).Return(nil)
	repo.On("FindJournalEntriesByRef", ctx, "EXP-2026-0001").Return([]domainFinance.JournalEntry{orig}, nil)
	repo.On("FindJournalEntryBySource", ctx, sourceExpenseReversal, uint(100)).Return(nil, appErrors.ErrNotFound)
	repo.On("FindJournalEntryBySource", ctx, sourceExpenseRevision, mock.Anything).Return(nil, appErrors.ErrNotFound)
	repo.On("GetAccountByCode", ctx, AccountOperatingExpense).Return(&domainFinance.Account{ID: 6, Code: AccountOperatingExpense, Name: "Operating Expense"}, nil)
	repo.On("GetAccountByCode", ctx, AccountCash).Return(&domainFinance.Account{ID: 1, Code: AccountCash, Name: "Cash"}, nil)

	var created []*domainFinance.JournalEntry
	repo.On("CreateJournalEntry", ctx, mock.Anything).Run(func(args mock.Arguments) {
		e := args.Get(1).(*domainFinance.JournalEntry)
		e.ID = uint(200 + len(created))
		created = append(created, e)
	}).Return(nil)

	exp, err := uc.UpdateExpense(ctx, 7, UpdateExpenseInput{Amount: 300, UpdatedBy: "owner@chawy.test"})
	require.NoError(t, err)
	assert.Equal(t, 300.0, exp.Amount)
	require.Len(t, created, 2, "expects one reversal and one revision entry")

	rev, revision := created[0], created[1]
	assert.Equal(t, sourceExpenseReversal, rev.SourceType)
	require.NotNil(t, rev.ReversalOfID)
	assert.Equal(t, uint(100), *rev.ReversalOfID)
	assert.Equal(t, orig.Date, rev.Date, "reversal is dated on the original date")
	assert.Equal(t, "owner@chawy.test", rev.CreatedBy)

	assert.Equal(t, sourceExpenseRevision, revision.SourceType)
	assert.Equal(t, rev.ID, revision.SourceID)
	assert.Equal(t, "EXP-2026-0001", revision.SourceRef)

	net := netByAccount(&orig, rev, revision)
	assert.InDelta(t, 300, net[AccountOperatingExpense], 0.001, "books now show the edited amount")
	assert.InDelta(t, -300, net[AccountCash], 0.001)
}

func TestUpdateExpense_TextOnlyChange_DoesNotTouchJournal(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	repo.On("GetExpenseByID", ctx, uint(7)).Return(activeExpense(500), nil)
	repo.On("UpdateExpense", ctx, mock.Anything).Return(nil)

	_, err := uc.UpdateExpense(ctx, 7, UpdateExpenseInput{Amount: 500, Description: "new note"})
	require.NoError(t, err)
	repo.AssertNotCalled(t, "CreateJournalEntry", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "FindJournalEntriesByRef", mock.Anything, mock.Anything)
}

func TestUpdateExpense_RejectsVoided(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	voided := activeExpense(500)
	voided.Status = domainFinance.ExpenseStatusVoided
	repo.On("GetExpenseByID", ctx, uint(7)).Return(voided, nil)

	_, err := uc.UpdateExpense(ctx, 7, UpdateExpenseInput{Amount: 300})
	assert.Error(t, err)
	repo.AssertNotCalled(t, "UpdateExpense", mock.Anything, mock.Anything)
}

func TestVoidExpense_ReversesLatestActiveEntry(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	// History: original(100) -> reversal(101) -> revision(102, amount 300). Only 102 is active.
	orig := postedExpenseJournal(100, 500)
	origID := uint(100)
	rev := domainFinance.JournalEntry{ID: 101, Code: orig.Code + "-R", SourceType: sourceExpenseReversal, SourceID: 100, SourceRef: orig.SourceRef, Status: domainFinance.JournalStatusPosted, ReversalOfID: &origID,
		Lines: []domainFinance.JournalLine{
			{AccountCode: AccountOperatingExpense, Credit: 500},
			{AccountCode: AccountCash, Debit: 500},
		}}
	revision := postedExpenseJournal(102, 300)
	revision.SourceType = sourceExpenseRevision
	revision.SourceID = 101
	revision.Code = "JE-2026/11/02-0002"

	repo.On("GetExpenseByID", ctx, uint(7)).Return(activeExpense(300), nil)
	repo.On("FindJournalEntriesByRef", ctx, "EXP-2026-0001").Return([]domainFinance.JournalEntry{orig, rev, revision}, nil)
	repo.On("FindJournalEntryBySource", ctx, sourceExpenseReversal, uint(102)).Return(nil, appErrors.ErrNotFound)

	var created *domainFinance.JournalEntry
	repo.On("CreateJournalEntry", ctx, mock.Anything).Run(func(args mock.Arguments) {
		created = args.Get(1).(*domainFinance.JournalEntry)
	}).Return(nil)

	var saved *domainFinance.Expense
	repo.On("UpdateExpense", ctx, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*domainFinance.Expense)
	}).Return(nil)

	exp, err := uc.VoidExpense(ctx, 7, "owner@chawy.test", "ซ้ำ")
	require.NoError(t, err)

	require.NotNil(t, created)
	require.NotNil(t, created.ReversalOfID)
	assert.Equal(t, uint(102), *created.ReversalOfID)

	net := netByAccount(&orig, &rev, &revision, created)
	assert.InDelta(t, 0, net[AccountOperatingExpense], 0.001, "voided expense nets to zero")
	assert.InDelta(t, 0, net[AccountCash], 0.001)

	require.NotNil(t, saved)
	assert.Equal(t, domainFinance.ExpenseStatusVoided, exp.Status)
	assert.Equal(t, "owner@chawy.test", exp.VoidedBy)
	assert.Equal(t, "ซ้ำ", exp.VoidReason)
	assert.NotNil(t, exp.VoidedAt)
}

func TestVoidExpense_AlreadyVoided(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	voided := activeExpense(500)
	voided.Status = domainFinance.ExpenseStatusVoided
	repo.On("GetExpenseByID", ctx, uint(7)).Return(voided, nil)

	_, err := uc.VoidExpense(ctx, 7, "owner@chawy.test", "")
	assert.Error(t, err)
	repo.AssertNotCalled(t, "CreateJournalEntry", mock.Anything, mock.Anything)
}

func TestTruncateRunes_KeepsThaiIntact(t *testing.T) {
	s := "กลับรายการค่าใช้จ่าย"
	out := truncateRunes(s, 5)
	assert.Equal(t, []rune(s)[:5], []rune(out))
}
