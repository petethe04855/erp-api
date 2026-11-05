package order_test

import (
	"context"
	"errors"
	"testing"

	domainOrder "chawy-erp-api/internal/domain/order"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecaseOrder "chawy-erp-api/internal/usecase/order"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSalesUsecase builds an order usecase with the shared test mocks and an
// empty stock repo (no stock operations are exercised here).
func newSalesUsecase(orderRepo *mockOrderRepo) usecaseOrder.Usecase {
	return usecaseOrder.NewOrderUsecaseWithTx(nil, orderRepo, newMockSKURepo(), newMockBundleRepo(), newMockFormulaRepo(), newMockStockRepo(), &fakeTxManager{})
}

// ListSales returns the orders with their (empty here, db is nil) invoice
// status and keeps the page clamping of the workspace screen.
func TestListSales_ReturnsOrdersWithInvoiceStatus(t *testing.T) {
	orderRepo := newMockOrderRepo()
	orderRepo.orders[1] = &domainOrder.Order{
		ID:           1,
		OrderNo:      "SO-2026-0001",
		CustomerName: "Alice",
		Status:       domainOrder.StatusPending,
		Items: []domainOrder.OrderItem{
			{SKU: "SKU-01", Quantity: 2, Price: 50},
		},
	}

	uc := newSalesUsecase(orderRepo)

	items, total, err := uc.ListSales(context.Background(), domainOrder.SalesQuery{Page: 1, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "SO-2026-0001", items[0].Order.OrderNo)
	assert.Equal(t, 1, len(items[0].Order.Items))
	assert.Empty(t, items[0].InvoiceStatus) // no invoice lookup without a db handle

	// Out-of-range limits clamp to the screen default of 50.
	items2, total2, err := uc.ListSales(context.Background(), domainOrder.SalesQuery{Page: 0, Limit: 500})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total2)
	assert.Len(t, items2, 1)
}

// UpdateStatusByRef returns a 404 application error carrying the historical
// message when the order cannot be resolved.
func TestUpdateStatusByRef_NotFound(t *testing.T) {
	uc := newSalesUsecase(newMockOrderRepo())

	_, already, err := uc.UpdateStatusByRef(context.Background(), "SO-MISSING", "CONFIRMED", "")
	require.Error(t, err)
	assert.False(t, already)

	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 404, appErr.StatusCode)
	assert.Equal(t, "Sales order not found", appErr.Message)
}

// Unknown status values are rejected with the historical message.
func TestUpdateStatusByRef_InvalidStatus(t *testing.T) {
	orderRepo := newMockOrderRepo()
	orderRepo.orders[1] = &domainOrder.Order{ID: 1, OrderNo: "SO-2026-0001", Status: domainOrder.StatusPending}
	uc := newSalesUsecase(orderRepo)

	_, already, err := uc.UpdateStatusByRef(context.Background(), "SO-2026-0001", "FOO", "")
	require.Error(t, err)
	assert.False(t, already)

	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 400, appErr.StatusCode)
	assert.Equal(t, "Invalid order status: FOO", appErr.Message)
}

// Same status is an idempotent no-op (already = true).
func TestUpdateStatusByRef_Idempotent(t *testing.T) {
	orderRepo := newMockOrderRepo()
	orderRepo.orders[1] = &domainOrder.Order{ID: 1, OrderNo: "SO-2026-0001", Status: domainOrder.StatusConfirmed}
	uc := newSalesUsecase(orderRepo)

	ord, already, err := uc.UpdateStatusByRef(context.Background(), "SO-2026-0001", "confirmed", "")
	require.NoError(t, err)
	assert.True(t, already)
	assert.Equal(t, domainOrder.StatusConfirmed, ord.Status)
}

// A plain status change stores the status (and note when supplied) through the
// repository.
func TestUpdateStatusByRef_SetsStatusAndNote(t *testing.T) {
	orderRepo := newMockOrderRepo()
	orderRepo.orders[1] = &domainOrder.Order{ID: 1, OrderNo: "SO-2026-0001", Status: domainOrder.StatusPending}
	uc := newSalesUsecase(orderRepo)

	ord, already, err := uc.UpdateStatusByRef(context.Background(), "SO-2026-0001", "CONFIRMED", "customer confirmed")
	require.NoError(t, err)
	assert.False(t, already)
	assert.Equal(t, domainOrder.StatusConfirmed, ord.Status)
	assert.Equal(t, "customer confirmed", ord.Note)

	assert.Equal(t, domainOrder.StatusConfirmed, orderRepo.orders[1].Status)
}

// Terminal orders cannot transition: the historical transition message is kept.
func TestUpdateStatusByRef_RejectsIllegalTransition(t *testing.T) {
	orderRepo := newMockOrderRepo()
	orderRepo.orders[1] = &domainOrder.Order{ID: 1, OrderNo: "SO-2026-0001", Status: domainOrder.StatusCancelled}
	uc := newSalesUsecase(orderRepo)

	_, _, err := uc.UpdateStatusByRef(context.Background(), "SO-2026-0001", "SHIPPED", "")
	require.Error(t, err)

	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, 400, appErr.StatusCode)
	assert.Equal(t, "Cannot transition order from CANCELLED to SHIPPED", appErr.Message)
}

// The stock dependency is only touched by ship/cancel paths; keep the compile
// assertion that the mock satisfies the full repository contract.
var _ domainOrder.Repository = (*mockOrderRepo)(nil)
var _ domainStock.Repository = (*mockStockRepo)(nil)
