package dto

type CreateExpenseRequest struct {
	Date        string  `json:"date"`
	Category    string  `json:"category"`
	Channel     string  `json:"channel"`
	Amount      float64 `json:"amount"`
	Vendor      string  `json:"vendor"`
	InvoiceRef  string  `json:"invoice_ref"`
	Description string  `json:"description"`
}

type UpdateExpenseRequest struct {
	Date        string  `json:"date"`
	Category    string  `json:"category"`
	Channel     string  `json:"channel"`
	Amount      float64 `json:"amount"`
	Vendor      string  `json:"vendor"`
	InvoiceRef  string  `json:"invoice_ref"`
	Description string  `json:"description"`
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
