package dto

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
