package dto

import (
	"time"

	domainFinance "chawy-erp-api/internal/domain/finance"
)

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

type JournalQueryRequest struct {
	From       string `json:"from"`
	To         string `json:"to"`
	SourceType string `json:"source_type"`
	Search     string `json:"search"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
}

type ExpenseQueryRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Category string `json:"category"`
	Channel  string `json:"channel"`
	Search   string `json:"search"`
	Page     int    `json:"page"`
	Limit    int    `json:"limit"`
}

// JournalLineResponse is the API contract for one debit/credit leg.
type JournalLineResponse struct {
	ID             uint      `json:"id"`
	JournalEntryID uint      `json:"journal_entry_id"`
	AccountID      uint      `json:"account_id"`
	AccountCode    string    `json:"account_code"`
	AccountName    string    `json:"account_name"`
	Debit          float64   `json:"debit"`
	Credit         float64   `json:"credit"`
	SKU            string    `json:"sku,omitempty"`
	Lot            string    `json:"lot,omitempty"`
	Channel        string    `json:"channel,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// JournalEntryResponse is the API contract for a posted journal entry.
type JournalEntryResponse struct {
	ID           uint                  `json:"id"`
	Code         string                `json:"code"`
	Date         string                `json:"date"`
	SourceType   string                `json:"source_type"`
	SourceID     uint                  `json:"source_id"`
	SourceRef    string                `json:"source_ref"`
	Description  string                `json:"description"`
	Status       domainFinance.JournalStatus `json:"status"`
	ReversalOfID *uint                 `json:"reversal_of_id,omitempty"`
	CreatedBy    string                `json:"created_by"`
	PostedAt     time.Time             `json:"posted_at"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	Lines        []JournalLineResponse `json:"lines"`
}

// NewJournalEntryResponse maps a domain journal entry to the API contract.
func NewJournalEntryResponse(e *domainFinance.JournalEntry) JournalEntryResponse {
	resp := JournalEntryResponse{
		ID:           e.ID,
		Code:         e.Code,
		Date:         e.Date,
		SourceType:   e.SourceType,
		SourceID:     e.SourceID,
		SourceRef:    e.SourceRef,
		Description:  e.Description,
		Status:       e.Status,
		ReversalOfID: e.ReversalOfID,
		CreatedBy:    e.CreatedBy,
		PostedAt:     e.PostedAt,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
	if e.Lines != nil {
		resp.Lines = make([]JournalLineResponse, len(e.Lines))
		for i, l := range e.Lines {
			resp.Lines[i] = JournalLineResponse{
				ID:             l.ID,
				JournalEntryID: l.JournalEntryID,
				AccountID:      l.AccountID,
				AccountCode:    l.AccountCode,
				AccountName:    l.AccountName,
				Debit:          l.Debit,
				Credit:         l.Credit,
				SKU:            l.SKU,
				Lot:            l.Lot,
				Channel:        l.Channel,
				CreatedAt:      l.CreatedAt,
				UpdatedAt:      l.UpdatedAt,
			}
		}
	}
	return resp
}

// NewJournalEntryResponses maps a slice of domain journal entries.
func NewJournalEntryResponses(items []domainFinance.JournalEntry) []JournalEntryResponse {
	out := make([]JournalEntryResponse, len(items))
	for i := range items {
		out[i] = NewJournalEntryResponse(&items[i])
	}
	return out
}

// ExpenseResponse is the API contract for an expense record.
type ExpenseResponse struct {
	ID          uint                     `json:"id"`
	Code        string                   `json:"code"`
	Date        string                   `json:"date"`
	Category    domainFinance.ExpenseCategory `json:"category"`
	Channel     domainFinance.ExpenseChannel  `json:"channel"`
	Amount      float64                  `json:"amount"`
	ProductName string                   `json:"product_name"`
	PurchaseURL string                   `json:"purchase_url"`
	SlipURL     string                   `json:"slip_url"`
	Vendor      string                   `json:"vendor"`
	InvoiceRef  string                   `json:"invoice_ref"`
	Description string                   `json:"description"`
	CreatedBy   string                   `json:"created_by"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

// NewExpenseResponse maps a domain expense to the API contract.
func NewExpenseResponse(e *domainFinance.Expense) ExpenseResponse {
	return ExpenseResponse{
		ID:          e.ID,
		Code:        e.Code,
		Date:        e.Date,
		Category:    e.Category,
		Channel:     e.Channel,
		Amount:      e.Amount,
		ProductName: e.ProductName,
		PurchaseURL: e.PurchaseURL,
		SlipURL:     e.SlipURL,
		Vendor:      e.Vendor,
		InvoiceRef:  e.InvoiceRef,
		Description: e.Description,
		CreatedBy:   e.CreatedBy,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// NewExpenseResponses maps a slice of domain expenses.
func NewExpenseResponses(items []domainFinance.Expense) []ExpenseResponse {
	out := make([]ExpenseResponse, len(items))
	for i := range items {
		out[i] = NewExpenseResponse(&items[i])
	}
	return out
}
