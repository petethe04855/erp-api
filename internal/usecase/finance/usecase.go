package finance

import (
	"context"

	domainFinance "chawy-erp-api/internal/domain/finance"
	"chawy-erp-api/pkg/database"
)

type PostingLineInput struct {
	AccountCode string  `json:"account_code"`
	Debit       float64 `json:"debit"`
	Credit      float64 `json:"credit"`
	SKU         string  `json:"sku,omitempty"`
	Lot         string  `json:"lot,omitempty"`
	Channel     string  `json:"channel,omitempty"`
}

type PostingRequest struct {
	Date        string             `json:"date"`
	SourceType  string             `json:"source_type"`
	SourceID    uint               `json:"source_id"`
	SourceRef   string             `json:"source_ref"`
	Description string             `json:"description"`
	CreatedBy   string             `json:"created_by"`
	Lines       []PostingLineInput `json:"lines"`
}

type CreateExpenseInput struct {
	Date        string                         `json:"date"`
	Category    domainFinance.ExpenseCategory  `json:"category"`
	Channel     domainFinance.ExpenseChannel   `json:"channel"`
	Amount      float64                        `json:"amount"`
	Vendor      string                         `json:"vendor"`
	InvoiceRef  string                         `json:"invoice_ref"`
	Description string                         `json:"description"`
	CreatedBy   string                         `json:"created_by"`
}

type UpdateExpenseInput struct {
	Date        string                         `json:"date"`
	Category    domainFinance.ExpenseCategory  `json:"category"`
	Channel     domainFinance.ExpenseChannel   `json:"channel"`
	Amount      float64                        `json:"amount"`
	Vendor      string                         `json:"vendor"`
	InvoiceRef  string                         `json:"invoice_ref"`
	Description string                         `json:"description"`
}

type GeneralLedgerRow struct {
	Date           string  `json:"date"`
	JournalCode    string  `json:"journal_code"`
	SourceType     string  `json:"source_type"`
	SourceRef      string  `json:"source_ref"`
	AccountCode    string  `json:"account_code"`
	AccountName    string  `json:"account_name"`
	Description    string  `json:"description"`
	SKU            string  `json:"sku,omitempty"`
	Lot            string  `json:"lot,omitempty"`
	Channel        string  `json:"channel,omitempty"`
	Debit          float64 `json:"debit"`
	Credit         float64 `json:"credit"`
	OpeningBalance float64 `json:"opening_balance"`
	RunningBalance float64 `json:"running_balance"`
}

type TrialBalanceRow struct {
	AccountCode   string  `json:"account_code"`
	AccountName   string  `json:"account_name"`
	AccountType   string  `json:"account_type"`
	OpeningDebit  float64 `json:"opening_debit"`
	OpeningCredit float64 `json:"opening_credit"`
	Debit         float64 `json:"debit"`
	Credit        float64 `json:"credit"`
	EndingDebit   float64 `json:"ending_debit"`
	EndingCredit  float64 `json:"ending_credit"`
}

type ProfitAndLossReport struct {
	From             string             `json:"from"`
	To               string             `json:"to"`
	Revenue          float64            `json:"revenue"`
	COGS             float64            `json:"cogs"`
	GrossProfit      float64            `json:"gross_profit"`
	OperatingExpense float64            `json:"operating_expense"`
	NetProfit        float64            `json:"net_profit"`
	RevenueByChannel map[string]float64 `json:"revenue_by_channel"`
	ExpenseByCategory map[string]float64 `json:"expense_by_category"`
}

type RevenueByChannelItem struct {
	Channel    string  `json:"channel"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type RevenueByChannelReport struct {
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Total      float64                `json:"total"`
	ByChannel  []RevenueByChannelItem `json:"by_channel"`
}

type Usecase interface {
	PostJournal(ctx context.Context, req PostingRequest) (*domainFinance.JournalEntry, error)
	ListJournalEntries(ctx context.Context, filter domainFinance.JournalFilter) ([]domainFinance.JournalEntry, int64, error)
	GetJournalEntryByID(ctx context.Context, id uint) (*domainFinance.JournalEntry, error)

	CreateExpense(ctx context.Context, in CreateExpenseInput) (*domainFinance.Expense, error)
	UpdateExpense(ctx context.Context, id uint, in UpdateExpenseInput) (*domainFinance.Expense, error)
	DeleteExpense(ctx context.Context, id uint) error
	GetExpenseByID(ctx context.Context, id uint) (*domainFinance.Expense, error)
	ListExpenses(ctx context.Context, filter domainFinance.ExpenseFilter) ([]domainFinance.Expense, int64, error)

	GetGeneralLedger(ctx context.Context, from, to string, accountID uint) ([]GeneralLedgerRow, error)
	GetTrialBalance(ctx context.Context, from, to string) ([]TrialBalanceRow, error)
	GetProfitAndLoss(ctx context.Context, from, to, channel string) (*ProfitAndLossReport, error)
	GetRevenueByChannel(ctx context.Context, from, to string) (*RevenueByChannelReport, error)
}

type financeUsecase struct {
	repo  domainFinance.Repository
	txMgr database.TxManager
}

func NewFinanceUsecase(repo domainFinance.Repository, txMgr database.TxManager) Usecase {
	return &financeUsecase{
		repo:  repo,
		txMgr: txMgr,
	}
}
