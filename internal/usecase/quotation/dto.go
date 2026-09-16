package quotation

// LineInput is a single quotation line as submitted by a client before
// validation and price/qty enforcement.
type LineInput struct {
	SKU       string
	Name      string
	Price     float64
	Qty       int
	ProductID uint
}

// CreateInput is the validated business input for creating a quotation.
type CreateInput struct {
	Customer   string
	Date       string
	ValidUntil string
	Status     string
	LeadSource string
	Note       string
	Lines      []LineInput
}

// ConversionResult reports the sales order created from a quotation.
type ConversionResult struct {
	QuotationID uint
	OrderID     uint
	OrderNo     string
}
