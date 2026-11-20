package finance

import (
	"context"
	"strings"
	"time"

	domainFinance "chawy-erp-api/internal/domain/finance"
	appErrors "chawy-erp-api/pkg/errors"
)

// CreateIncome records an income slip uploaded from the LINE OA / LIFF app.
// The business defaults mirror the legacy erp-expense-api behaviour, except for
// the category default ("รายได้อื่นๆ" instead of "ยอดขาย", which is not part of
// the dashboard's income category enum).
func (u *financeUsecase) CreateIncome(ctx context.Context, in CreateIncomeInput) (*domainFinance.Income, error) {
	if in.Amount <= 0 {
		return nil, appErrors.NewAppError("INVALID_AMOUNT", "Income amount must be greater than zero", 400)
	}

	dateStr := strings.TrimSpace(in.Date)
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	category := domainFinance.IncomeCategory(strings.TrimSpace(string(in.Category)))
	if category == "" {
		category = domainFinance.IncomeCategoryOther
	}

	channel := domainFinance.ExpenseChannel(strings.TrimSpace(string(in.Channel)))
	if channel == "" {
		channel = domainFinance.ExpenseChannelGeneral
	}

	createdBy := strings.TrimSpace(in.CreatedBy)
	if createdBy == "" {
		createdBy = "system"
	}

	code, err := u.nextCode(ctx, "INC", dateStr)
	if err != nil {
		return nil, err
	}

	inc := &domainFinance.Income{
		Code:        code,
		Date:        dateStr,
		Category:    category,
		Channel:     channel,
		Amount:      in.Amount,
		ProductName: clip(in.ProductName, maxProductName),
		PurchaseURL: in.PurchaseURL,
		SlipURL:     in.SlipURL,
		Vendor:      clip(in.Vendor, maxVendor),
		InvoiceRef:  clip(in.InvoiceRef, maxInvoiceRef),
		Description: in.Description,
		CreatedBy:   clip(createdBy, maxCreatedBy),
	}

	// TODO(finance): post income journal
	if err := u.repo.CreateIncome(ctx, inc); err != nil {
		return nil, err
	}
	return inc, nil
}
