package invoice_test

import (
	"context"
	"testing"

	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	usecaseInvoice "chawy-erp-api/internal/usecase/invoice"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
)

type mockInvoiceRepo struct {
	invoices []*domainInvoice.Invoice
}

func (m *mockInvoiceRepo) Create(ctx context.Context, inv *domainInvoice.Invoice) error {
	inv.ID = uint(len(m.invoices) + 1)
	m.invoices = append(m.invoices, inv)
	return nil
}

func (m *mockInvoiceRepo) FindByID(ctx context.Context, id uint) (*domainInvoice.Invoice, error) {
	for _, inv := range m.invoices {
		if inv.ID == id {
			return inv, nil
		}
	}
	return nil, nil
}

func (m *mockInvoiceRepo) FindByIDForUpdate(ctx context.Context, id uint) (*domainInvoice.Invoice, error) {
	return m.FindByID(ctx, id)
}

func (m *mockInvoiceRepo) FindByOrderID(ctx context.Context, orderID uint) (*domainInvoice.Invoice, error) {
	for _, inv := range m.invoices {
		if inv.OrderID != nil && *inv.OrderID == orderID {
			return inv, nil
		}
	}
	return nil, nil
}

func (m *mockInvoiceRepo) FindByInvoiceNo(ctx context.Context, no string) (*domainInvoice.Invoice, error) {
	for _, inv := range m.invoices {
		if inv.InvoiceNo == no {
			return inv, nil
		}
	}
	return nil, nil
}

func (m *mockInvoiceRepo) FindAll(ctx context.Context, q domainInvoice.Query) ([]domainInvoice.Invoice, int64, error) {
	var res []domainInvoice.Invoice
	for _, inv := range m.invoices {
		res = append(res, *inv)
	}
	return res, int64(len(res)), nil
}

func (m *mockInvoiceRepo) Update(ctx context.Context, inv *domainInvoice.Invoice) error {
	for i, existing := range m.invoices {
		if existing.ID == inv.ID {
			m.invoices[i] = inv
			return nil
		}
	}
	return nil
}

type mockOrderRepo struct {
	orders []*domainOrder.Order
}

func (m *mockOrderRepo) Create(ctx context.Context, order *domainOrder.Order) error {
	order.ID = uint(len(m.orders) + 1)
	m.orders = append(m.orders, order)
	return nil
}

func (m *mockOrderRepo) FindByID(ctx context.Context, id uint) (*domainOrder.Order, error) {
	for _, o := range m.orders {
		if o.ID == id {
			return o, nil
		}
	}
	return nil, nil
}

func (m *mockOrderRepo) FindByIDForUpdate(ctx context.Context, id uint) (*domainOrder.Order, error) {
	return m.FindByID(ctx, id)
}

func (m *mockOrderRepo) FindByOrderNo(ctx context.Context, orderNo string) (*domainOrder.Order, error) {
	for _, o := range m.orders {
		if o.OrderNo == orderNo {
			return o, nil
		}
	}
	return nil, nil
}

func (m *mockOrderRepo) FindAll(ctx context.Context, q domainOrder.Query) ([]domainOrder.Order, int64, error) {
	var res []domainOrder.Order
	for _, o := range m.orders {
		res = append(res, *o)
	}
	return res, int64(len(res)), nil
}

func (m *mockOrderRepo) UpdateStatus(ctx context.Context, id uint, status domainOrder.Status) error {
	for _, o := range m.orders {
		if o.ID == id {
			o.Status = status
			return nil
		}
	}
	return nil
}

func (m *mockOrderRepo) Update(ctx context.Context, order *domainOrder.Order) error {
	for i, existing := range m.orders {
		if existing.ID == order.ID {
			m.orders[i] = order
			return nil
		}
	}
	return nil
}

func (m *mockOrderRepo) Delete(ctx context.Context, id uint) error {
	return nil
}

func TestInvoiceUsecase_CreateFromOrder_Idempotency(t *testing.T) {
	orderID := uint(100)
	order := &domainOrder.Order{
		ID:           orderID,
		OrderNo:      "SO-2026-0001",
		CustomerName: "Customer A",
		TotalAmount:  500.0,
		Status:       domainOrder.StatusConfirmed,
	}

	orderRepo := &mockOrderRepo{orders: []*domainOrder.Order{order}}
	invRepo := &mockInvoiceRepo{}
	uc := usecaseInvoice.NewInvoiceUsecase(invRepo, orderRepo)

	// First creation
	inv1, err := uc.CreateFromOrder(context.Background(), usecaseInvoice.CreateInvoiceFromOrderInput{
		OrderID: orderID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, inv1)
	assert.Equal(t, uint(1), inv1.ID)
	assert.Equal(t, 500.0, inv1.Amount)

	// Second creation for same order -> must return existing invoice idempotently without creating new row
	inv2, err := uc.CreateFromOrder(context.Background(), usecaseInvoice.CreateInvoiceFromOrderInput{
		OrderID: orderID,
	})
	assert.NoError(t, err)
	assert.NotNil(t, inv2)
	assert.Equal(t, inv1.ID, inv2.ID)
	assert.Equal(t, 1, len(invRepo.invoices))
}

func TestInvoiceUsecase_MarkAsPaid_NegativeAmountValidation(t *testing.T) {
	orderID := uint(101)
	inv := &domainInvoice.Invoice{
		ID:           1,
		OrderID:      &orderID,
		OrderNo:      "SO-2026-0002",
		CustomerName: "Customer B",
		Amount:       1000.0,
		PaidAmount:   0,
		Status:       domainInvoice.StatusUnpaid,
	}

	orderRepo := &mockOrderRepo{}
	invRepo := &mockInvoiceRepo{invoices: []*domainInvoice.Invoice{inv}}
	uc := usecaseInvoice.NewInvoiceUsecase(invRepo, orderRepo)

	// Negative amount should fail with 400 Bad Request
	_, err := uc.MarkAsPaid(context.Background(), usecaseInvoice.MarkPaidInput{
		InvoiceID: 1,
		Amount:    -100.0,
	})
	assert.Error(t, err)
	appErr, ok := err.(*appErrors.AppError)
	assert.True(t, ok)
	assert.Equal(t, 400, appErr.StatusCode)
	assert.Equal(t, "INVALID_AMOUNT", appErr.Code)

	// Amount == 0 should pay the remaining balance in full
	paidInv, err := uc.MarkAsPaid(context.Background(), usecaseInvoice.MarkPaidInput{
		InvoiceID: 1,
		Amount:    0,
	})
	assert.NoError(t, err)
	assert.Equal(t, domainInvoice.StatusPaid, paidInv.Status)
	assert.Equal(t, 1000.0, paidInv.PaidAmount)
}
