package customer

import (
	"context"
	"errors"
	"strings"
	"testing"

	domainCustomer "chawy-erp-api/internal/domain/customer"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wsFake extends the base fakeRepo with the workspace read contract so the
// constructor's type-assertion probe picks it up.
type wsFake struct {
	*fakeRepo

	wsItems []domainCustomer.Customer
	wsTotal int64
	wsQuery domainCustomer.WorkspaceQuery

	findByIDC *domainCustomer.Customer
	findByIDE error
	updateErr error
}

func (w *wsFake) FindAllForWorkspace(_ context.Context, q domainCustomer.WorkspaceQuery) ([]domainCustomer.Customer, int64, error) {
	w.wsQuery = q
	return w.wsItems, w.wsTotal, nil
}

// FindByID routes through the injected fixture so tests can simulate
// record-not-found (nil, nil) and real database failures (err != nil).
func (w *wsFake) FindByID(_ context.Context, id uint) (*domainCustomer.Customer, error) {
	return w.findByIDC, w.findByIDE
}

// Update routes through the injected failure so tests can simulate a failed save.
func (w *wsFake) Update(_ context.Context, c *domainCustomer.Customer) error {
	w.last = c
	return w.updateErr
}

func newWsUsecase(w *wsFake) Usecase {
	return NewCustomerUsecase(w)
}

func TestCreateForWorkspaceStampsCodeAndKeepsProvinceEmpty(t *testing.T) {
	ws := &wsFake{fakeRepo: &fakeRepo{}}
	cust, err := newWsUsecase(ws).CreateForWorkspace(context.Background(), WorkspaceCreateInput{
		Name:          "ABC Co., Ltd.",
		ContactPerson: "Somchai",
		Email:         "somchai@example.com",
		Phone:         "081-000-0000",
		TaxID:         "1234567890",
		Address:       "123 ห้วยขวาง กรุงเทพมหานคร 10310",
	})
	require.NoError(t, err)
	require.NotNil(t, cust)

	assert.True(t, strings.HasPrefix(cust.Code, "CUST-"), "code must use the CUST- timestamp prefix")
	assert.Len(t, cust.Code, len("CUST-")+len("20060102150405"))
	assert.Equal(t, "ABC Co., Ltd.", cust.Name)
	assert.Equal(t, "Somchai", cust.ContactPerson)
	assert.Equal(t, "active", cust.Status)
	// Workspace create never derives a province from the address (historical
	// behavior of the inline handler; only the module-level Create does).
	assert.Empty(t, cust.Province)
	assert.False(t, cust.CreatedAt.IsZero())
	assert.False(t, cust.UpdatedAt.IsZero())

	// The created row reached the repository.
	require.NotNil(t, ws.last)
	assert.Equal(t, cust.Code, ws.last.Code)
}

func TestListForWorkspacePassesQueryAndUsesRepository(t *testing.T) {
	ws := &wsFake{
		fakeRepo: &fakeRepo{},
		wsItems:  []domainCustomer.Customer{{ID: 3, Name: "Foo Ltd"}},
		wsTotal:  7,
	}
	uc := newWsUsecase(ws)
	items, total, err := uc.ListForWorkspace(context.Background(), domainCustomer.WorkspaceQuery{Page: 2, Limit: 10, Search: "foo"})
	require.NoError(t, err)
	assert.Equal(t, 7, int(total))
	assert.Len(t, items, 1)
	assert.Equal(t, domainCustomer.WorkspaceQuery{Page: 2, Limit: 10, Search: "foo"}, ws.wsQuery)
}

func TestSetStatusForWorkspaceNotFoundBecomes404(t *testing.T) {
	ws := &wsFake{fakeRepo: &fakeRepo{}} // FindByID returns nil, nil (record missing)
	_, err := newWsUsecase(ws).SetStatusForWorkspace(context.Background(), 9, "inactive")
	var appE *appErrors.AppError
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, 404, appE.StatusCode)
	assert.Equal(t, "Customer not found", appE.Message)
}

func TestSetStatusForWorkspaceDBFailureBecomes500(t *testing.T) {
	ws := &wsFake{
		fakeRepo:  &fakeRepo{},
		findByIDC: &domainCustomer.Customer{ID: 9, Status: "active"},
		updateErr: errors.New("db write failed"),
	}
	_, err := newWsUsecase(ws).SetStatusForWorkspace(context.Background(), 9, "inactive")
	var appE *appErrors.AppError
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, 500, appE.StatusCode)
	assert.Equal(t, "Failed to update customer status", appE.Message)
}

func TestSetStatusForWorkspaceAppliesRawStatus(t *testing.T) {
	ws := &wsFake{
		fakeRepo:  &fakeRepo{},
		findByIDC: &domainCustomer.Customer{ID: 9, Status: "active"},
	}
	cust, err := newWsUsecase(ws).SetStatusForWorkspace(context.Background(), 9, "inactive")
	require.NoError(t, err)
	assert.Equal(t, "inactive", cust.Status)
	assert.False(t, cust.UpdatedAt.IsZero())
	require.NotNil(t, ws.last)
	assert.Equal(t, "inactive", ws.last.Status)
}

// 404 mapping must also fire for a real db error on the initial load, matching
// the original inline handler (any First()-error => "Customer not found").
func TestSetStatusForWorkspaceLoadErrorAlso404(t *testing.T) {
	ws := &wsFake{
		fakeRepo:  &fakeRepo{},
		findByIDE: errors.New("connection refused"),
	}
	_, err := newWsUsecase(ws).SetStatusForWorkspace(context.Background(), 9, "inactive")
	var appE *appErrors.AppError
	require.ErrorAs(t, err, &appE)
	assert.Equal(t, 404, appE.StatusCode)
	assert.Equal(t, "Customer not found", appE.Message)
}
