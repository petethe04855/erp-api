package invoice_test

import (
	"context"
	"testing"

	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	usecaseInvoice "chawy-erp-api/internal/usecase/invoice"

	"github.com/stretchr/testify/assert"
)

// paymentRaceRepo serializes FindByIDForUpdate/Update through the same mutex
// the real DB row lock provides, so two payments cannot read the same snapshot.
type paymentRaceRepo struct {
	*mockInvoiceRepo
	seq chan struct{}
}

func (r *paymentRaceRepo) FindByIDForUpdate(ctx context.Context, id uint) (*domainInvoice.Invoice, error) {
	<-r.seq // acquire the "row lock"
	return r.mockInvoiceRepo.FindByID(ctx, id)
}

func (r *paymentRaceRepo) Update(ctx context.Context, inv *domainInvoice.Invoice) error {
	err := r.mockInvoiceRepo.Update(ctx, inv)
	r.seq <- struct{}{} // release the "row lock"
	return err
}

// FULL-18: two payments of 300 and 400 on a 1000 invoice must accumulate to
// 700 paid / PARTIALLY_PAID — not the 400 lost-update result the review found.
func TestMarkAsPaid_ConcurrentStylePaymentsAccumulate(t *testing.T) {
	orderID := uint(200)
	inv := &domainInvoice.Invoice{
		ID:           1,
		OrderID:      &orderID,
		OrderNo:      "SO-2026-RACE",
		CustomerName: "Customer C",
		Amount:       1000.0,
		PaidAmount:   0,
		Status:       domainInvoice.StatusUnpaid,
	}

	mockInv := &mockInvoiceRepo{invoices: []*domainInvoice.Invoice{inv}}
	repo := &paymentRaceRepo{mockInvoiceRepo: mockInv, seq: make(chan struct{}, 1)}
	repo.seq <- struct{}{}

	orderRepo := &mockOrderRepo{}
	uc := usecaseInvoice.NewInvoiceUsecase(repo, orderRepo)

	pay1 := make(chan error, 1)
	go func() {
		_, err := uc.MarkAsPaid(context.Background(), usecaseInvoice.MarkPaidInput{InvoiceID: 1, Amount: 300})
		pay1 <- err
	}()

	pay2Err := func() error {
		_, err := uc.MarkAsPaid(context.Background(), usecaseInvoice.MarkPaidInput{InvoiceID: 1, Amount: 400})
		return err
	}()

	err1 := <-pay1
	assert.NoError(t, err1)
	assert.NoError(t, pay2Err)

	stored := mockInv.invoices[0]
	assert.Equal(t, 700.0, stored.PaidAmount)
	assert.Equal(t, domainInvoice.StatusPartiallyPaid, stored.Status)
}

// The order repository mock must satisfy the interface extension.
var _ domainOrder.Repository = (*mockOrderRepo)(nil)
