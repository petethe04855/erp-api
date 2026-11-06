package finance

import (
	"context"
	"testing"

	domainFinance "chawy-erp-api/internal/domain/finance"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockFinanceRepo struct {
	mock.Mock
}

func (m *mockFinanceRepo) GetAccountByCode(ctx context.Context, code string) (*domainFinance.Account, error) {
	args := m.Called(ctx, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainFinance.Account), args.Error(1)
}

func (m *mockFinanceRepo) ListAccounts(ctx context.Context) ([]domainFinance.Account, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domainFinance.Account), args.Error(1)
}

func (m *mockFinanceRepo) GetAccountMapping(ctx context.Context, key string) (*domainFinance.AccountMapping, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainFinance.AccountMapping), args.Error(1)
}

func (m *mockFinanceRepo) ListAccountMappings(ctx context.Context) ([]domainFinance.AccountMapping, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domainFinance.AccountMapping), args.Error(1)
}

func (m *mockFinanceRepo) UpsertAccountMapping(ctx context.Context, mapping *domainFinance.AccountMapping) error {
	args := m.Called(ctx, mapping)
	return args.Error(0)
}

func (m *mockFinanceRepo) CreateJournalEntry(ctx context.Context, entry *domainFinance.JournalEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *mockFinanceRepo) FindJournalEntries(ctx context.Context, filter domainFinance.JournalFilter) ([]domainFinance.JournalEntry, int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domainFinance.JournalEntry), args.Get(1).(int64), args.Error(2)
}

func (m *mockFinanceRepo) FindJournalEntryByID(ctx context.Context, id uint) (*domainFinance.JournalEntry, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainFinance.JournalEntry), args.Error(1)
}

func (m *mockFinanceRepo) FindJournalEntryBySource(ctx context.Context, sourceType string, sourceID uint) (*domainFinance.JournalEntry, error) {
	args := m.Called(ctx, sourceType, sourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainFinance.JournalEntry), args.Error(1)
}

func (m *mockFinanceRepo) GetJournalLinesForReport(ctx context.Context, filter domainFinance.ReportFilter) ([]domainFinance.JournalLine, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domainFinance.JournalLine), args.Error(1)
}

func (m *mockFinanceRepo) GetOpeningBalances(ctx context.Context, beforeDate string) (map[string]float64, error) {
	args := m.Called(ctx, beforeDate)
	return args.Get(0).(map[string]float64), args.Error(1)
}

func (m *mockFinanceRepo) CreateExpense(ctx context.Context, exp *domainFinance.Expense) error {
	args := m.Called(ctx, exp)
	return args.Error(0)
}

func (m *mockFinanceRepo) UpdateExpense(ctx context.Context, exp *domainFinance.Expense) error {
	args := m.Called(ctx, exp)
	return args.Error(0)
}

func (m *mockFinanceRepo) DeleteExpense(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockFinanceRepo) GetExpenseByID(ctx context.Context, id uint) (*domainFinance.Expense, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainFinance.Expense), args.Error(1)
}

func (m *mockFinanceRepo) ListExpenses(ctx context.Context, filter domainFinance.ExpenseFilter) ([]domainFinance.Expense, int64, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]domainFinance.Expense), args.Get(1).(int64), args.Error(2)
}

func TestValidatePostingLines(t *testing.T) {
	t.Run("fails on less than 2 lines", func(t *testing.T) {
		err := validatePostingLines([]PostingLineInput{
			{AccountCode: "1100", Debit: 100},
		})
		assert.Error(t, err)
	})

	t.Run("fails on unbalanced debit credit", func(t *testing.T) {
		err := validatePostingLines([]PostingLineInput{
			{AccountCode: "1100", Debit: 100},
			{AccountCode: "4000", Credit: 80},
		})
		assert.Error(t, err)
	})

	t.Run("passes on balanced entry", func(t *testing.T) {
		err := validatePostingLines([]PostingLineInput{
			{AccountCode: "1100", Debit: 100},
			{AccountCode: "4000", Credit: 100},
		})
		assert.NoError(t, err)
	})
}

func TestPostJournal_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	repo.On("FindJournalEntryBySource", ctx, "order_delivery", uint(10)).Return(nil, nil)
	repo.On("GetAccountByCode", ctx, "5000").Return(&domainFinance.Account{ID: 5, Code: "5000", Name: "COGS"}, nil)
	repo.On("GetAccountByCode", ctx, "1300").Return(&domainFinance.Account{ID: 2, Code: "1300", Name: "Inventory"}, nil)
	repo.On("CreateJournalEntry", ctx, mock.AnythingOfType("*finance.JournalEntry")).Return(nil)

	entry, err := uc.PostJournal(ctx, PostingRequest{
		Date:        "2026-09-11",
		SourceType:  "order_delivery",
		SourceID:    10,
		SourceRef:   "SO-001",
		Description: "COGS for order",
		CreatedBy:   "tester",
		Lines: []PostingLineInput{
			{AccountCode: "5000", Debit: 500},
			{AccountCode: "1300", Credit: 500},
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, entry)
	assert.Equal(t, 2, len(entry.Lines))
	repo.AssertExpectations(t)
}
