package finance

import (
	"context"
)

type JournalFilter struct {
	From       string
	To         string
	SourceType string
	Search     string
	Page       int
	Limit      int
}

type ExpenseFilter struct {
	From     string
	To       string
	Category string
	Channel  string
	Search   string
	Page     int
	Limit    int
}

type ReportFilter struct {
	From      string
	To        string
	AccountID uint
	Channel   string
}

type Repository interface {
	// Accounts
	GetAccountByCode(ctx context.Context, code string) (*Account, error)
	ListAccounts(ctx context.Context) ([]Account, error)
	GetAccountMapping(ctx context.Context, key string) (*AccountMapping, error)
	ListAccountMappings(ctx context.Context) ([]AccountMapping, error)
	UpsertAccountMapping(ctx context.Context, mapping *AccountMapping) error

	// Journal
	CreateJournalEntry(ctx context.Context, entry *JournalEntry) error
	FindJournalEntries(ctx context.Context, filter JournalFilter) ([]JournalEntry, int64, error)
	FindJournalEntryByID(ctx context.Context, id uint) (*JournalEntry, error)
	FindJournalEntryBySource(ctx context.Context, sourceType string, sourceID uint) (*JournalEntry, error)
	GetJournalLinesForReport(ctx context.Context, filter ReportFilter) ([]JournalLine, error)
	GetOpeningBalances(ctx context.Context, beforeDate string) (map[string]float64, error)

	// Expenses
	CreateExpense(ctx context.Context, exp *Expense) error
	UpdateExpense(ctx context.Context, exp *Expense) error
	DeleteExpense(ctx context.Context, id uint) error
	GetExpenseByID(ctx context.Context, id uint) (*Expense, error)
	ListExpenses(ctx context.Context, filter ExpenseFilter) ([]Expense, int64, error)
}
