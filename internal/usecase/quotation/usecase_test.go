package quotation_test

import (
	"context"
	"strings"
	"testing"

	domainOrder "chawy-erp-api/internal/domain/order"
	domainQuotation "chawy-erp-api/internal/domain/quotation"
	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseQuotation "chawy-erp-api/internal/usecase/quotation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeQuotationRepo struct {
	quotation *domainQuotation.Quotation
	created   *domainQuotation.Quotation
	saved     *domainQuotation.Quotation
}

func (f *fakeQuotationRepo) Create(ctx context.Context, q *domainQuotation.Quotation) error {
	cp := *q
	f.created = &cp
	f.quotation = &cp
	q.ID = 1
	return nil
}

func (f *fakeQuotationRepo) FindByID(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	if f.quotation != nil && f.quotation.ID == id {
		cp := *f.quotation
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeQuotationRepo) FindByIDWithLines(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	return f.FindByID(ctx, id)
}

func (f *fakeQuotationRepo) FindByIDForUpdate(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	return f.FindByID(ctx, id)
}

func (f *fakeQuotationRepo) FindByIDForUpdateWithLines(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	return f.FindByID(ctx, id)
}

func (f *fakeQuotationRepo) List(ctx context.Context, page, limit int) ([]domainQuotation.Quotation, int64, error) {
	return nil, 0, nil
}

func (f *fakeQuotationRepo) Save(ctx context.Context, q *domainQuotation.Quotation) error {
	cp := *q
	f.saved = &cp
	f.quotation = &cp
	return nil
}

func (f *fakeQuotationRepo) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

type fakeSKURepo struct {
	sku *domainSKU.SKU
}

func (f *fakeSKURepo) FindBySKU(ctx context.Context, code string) (*domainSKU.SKU, error) {
	if f.sku != nil && strings.EqualFold(strings.TrimSpace(f.sku.SKU), code) {
		cp := *f.sku
		return &cp, nil
	}
	return nil, nil
}

type fakeOrderRepo struct {
	created *domainOrder.Order
}

func (f *fakeOrderRepo) Create(ctx context.Context, order *domainOrder.Order) error {
	cp := *order
	f.created = &cp
	order.ID = 99
	return nil
}

func newUsecase(repo *fakeQuotationRepo, sku *fakeSKURepo) usecaseQuotation.Usecase {
	return usecaseQuotation.NewQuotationUsecase(repo, sku, &fakeOrderRepo{}, repo)
}

func TestCreate_RejectsNonPositiveQuantity(t *testing.T) {
	uc := newUsecase(&fakeQuotationRepo{}, &fakeSKURepo{sku: &domainSKU.SKU{ID: 1, SKU: "ABC"}})

	_, err := uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer: "Acme",
		Lines: []domainQuotation.LineInput{
			{SKU: "ABC", Price: 100, Qty: 0},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "quantity must be positive")
}

func TestCreate_RejectsNegativePrice(t *testing.T) {
	uc := newUsecase(&fakeQuotationRepo{}, &fakeSKURepo{sku: &domainSKU.SKU{ID: 1, SKU: "ABC"}})

	_, err := uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer: "Acme",
		Lines: []domainQuotation.LineInput{
			{SKU: "ABC", Price: -5, Qty: 1},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "price must not be negative")
}

func TestCreate_RejectsUnknownSKU(t *testing.T) {
	uc := newUsecase(&fakeQuotationRepo{}, &fakeSKURepo{})

	_, err := uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer: "Acme",
		Lines: []domainQuotation.LineInput{
			{SKU: "NOPE", Price: 10, Qty: 1},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCreate_RejectsProductSKUMismatch(t *testing.T) {
	uc := newUsecase(&fakeQuotationRepo{}, &fakeSKURepo{sku: &domainSKU.SKU{ID: 1, SKU: "ABC"}})

	_, err := uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer: "Acme",
		Lines: []domainQuotation.LineInput{
			{SKU: "ABC", ProductID: 42, Price: 10, Qty: 1},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match SKU")
}

func TestCreate_RejectsNonDraftStatus(t *testing.T) {
	uc := newUsecase(&fakeQuotationRepo{}, &fakeSKURepo{sku: &domainSKU.SKU{ID: 1, SKU: "ABC"}})

	// Attempting to create with an invalid status string
	_, err := uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer: "Acme",
		Status:   "Nonsense",
		Lines: []domainQuotation.LineInput{
			{SKU: "ABC", Price: 10, Qty: 1},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "New quotations must be created with status 'Draft'")

	// Attempting to bypass state machine by creating as Approved directly
	_, err = uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer: "Acme",
		Status:   string(domainQuotation.StatusApproved),
		Lines: []domainQuotation.LineInput{
			{SKU: "ABC", Price: 10, Qty: 1},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "New quotations must be created with status 'Draft'")

	// Attempting to create as Converted directly
	_, err = uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer: "Acme",
		Status:   string(domainQuotation.StatusConverted),
		Lines: []domainQuotation.LineInput{
			{SKU: "ABC", Price: 10, Qty: 1},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "New quotations must be created with status 'Draft'")
}

func TestCreate_RejectsValidUntilBeforeDate(t *testing.T) {
	uc := newUsecase(&fakeQuotationRepo{}, &fakeSKURepo{sku: &domainSKU.SKU{ID: 1, SKU: "ABC"}})

	_, err := uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer:   "Acme",
		Date:       "2026-09-10",
		ValidUntil: "2026-09-01",
		Lines: []domainQuotation.LineInput{
			{SKU: "ABC", Price: 10, Qty: 1},
		},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validUntil must not be before date")
}

func TestCreate_ComputesTotalFromValidatedLines(t *testing.T) {
	repo := &fakeQuotationRepo{}
	uc := newUsecase(repo, &fakeSKURepo{sku: &domainSKU.SKU{ID: 1, SKU: "ABC"}})

	q, err := uc.Create(context.Background(), domainQuotation.CreateInput{
		Customer: "Acme",
		Lines: []domainQuotation.LineInput{
			{SKU: "abc", Price: 100, Qty: 2}, // lowercase SKU must normalize
			{SKU: "ABC", Price: 50, Qty: 1},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, 250.0, q.TotalAmount)
	assert.Equal(t, uint(1), q.Lines[0].ProductID)
	assert.Equal(t, "ABC", q.Lines[0].SKU)
	assert.Equal(t, domainQuotation.StatusDraft, q.Status)
}

func TestUpdateStatus_EnforcesStateMachine(t *testing.T) {
	repo := &fakeQuotationRepo{quotation: &domainQuotation.Quotation{
		ID:     1,
		Status: domainQuotation.StatusDraft,
	}}
	uc := newUsecase(repo, &fakeSKURepo{})

	// Draft -> Approved is not a permitted transition.
	_, err := uc.UpdateStatus(context.Background(), 1, domainQuotation.StatusApproved)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot change quotation status")

	// Draft -> Sent is permitted.
	updated, err := uc.UpdateStatus(context.Background(), 1, domainQuotation.StatusSent)
	require.NoError(t, err)
	assert.Equal(t, domainQuotation.StatusSent, updated.Status)
}

func TestConvert_OnlyApprovedQuotationsConvert(t *testing.T) {
	orderRepo := &fakeOrderRepo{}
	repo := &fakeQuotationRepo{
		quotation: &domainQuotation.Quotation{
			ID:         1,
			Status:     domainQuotation.StatusDraft,
			ValidUntil: "2099-01-01",
			Lines: []domainQuotation.QuotationLine{
				{SKU: "ABC", Price: 100, Quantity: 1, Subtotal: 100},
			},
		},
	}
	uc := usecaseQuotation.NewQuotationUsecase(repo, &fakeSKURepo{}, orderRepo, repo)

	_, err := uc.ConvertToSalesOrder(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Only Approved quotations")
	assert.Nil(t, orderRepo.created)
}

func TestConvert_CreatesPendingSalesOrderAndMarksConverted(t *testing.T) {
	orderRepo := &fakeOrderRepo{}
	repo := &fakeQuotationRepo{
		quotation: &domainQuotation.Quotation{
			ID:           1,
			Code:         "QT-2026-0001",
			CustomerName: "Acme",
			Status:       domainQuotation.StatusApproved,
			ValidUntil:   "2099-01-01",
			TotalAmount:  100,
			Lines: []domainQuotation.QuotationLine{
				{SKU: "ABC", Price: 100, Quantity: 1, Subtotal: 100},
			},
		},
	}
	uc := usecaseQuotation.NewQuotationUsecase(repo, &fakeSKURepo{}, orderRepo, repo)

	result, err := uc.ConvertToSalesOrder(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, orderRepo.created)
	assert.Equal(t, uint(99), result.OrderID)
	assert.Equal(t, domainOrder.StatusPending, orderRepo.created.Status)
	assert.Equal(t, domainQuotation.StatusConverted, repo.quotation.Status)
}

func TestConvert_RejectsExpiredQuotation(t *testing.T) {
	orderRepo := &fakeOrderRepo{}
	repo := &fakeQuotationRepo{
		quotation: &domainQuotation.Quotation{
			ID:         1,
			Status:     domainQuotation.StatusApproved,
			ValidUntil: "2020-01-01",
			Lines: []domainQuotation.QuotationLine{
				{SKU: "ABC", Price: 100, Quantity: 1, Subtotal: 100},
			},
		},
	}
	uc := usecaseQuotation.NewQuotationUsecase(repo, &fakeSKURepo{}, orderRepo, repo)

	_, err := uc.ConvertToSalesOrder(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
	assert.Nil(t, orderRepo.created)
}
