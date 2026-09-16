package dto

type CreateInvoiceRequest struct {
	OrderID uint `json:"order_id"`
}

type MarkPaidRequest struct {
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	Method        string  `json:"method"`
	AccountCode   string  `json:"account_code"`
	AccountCode2  string  `json:"accountCode"`
}
