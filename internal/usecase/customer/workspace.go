package customer

import (
	"context"
	"fmt"
	"time"

	domainCustomer "chawy-erp-api/internal/domain/customer"
	appErrors "chawy-erp-api/pkg/errors"
)

// Workspace methods for the erp-web-v2 workspace endpoints. These keep the
// historical behavior of the inline handler code (which differed subtly from
// the standard customer module):
//   - ListWorkspace sorts/paginates the same way but its search ALSO matches
//     contact_person (not the code).
//   - CreateWorkspace stamps the CUST-timestamp code, sets status active and
//     leaves province empty (no address→province derivation).
type WorkspaceCreateInput struct {
	Name          string
	ContactPerson string
	Email         string
	Phone         string
	TaxID         string
	Address       string
	Logo          string
}

// ListForWorkspace lists customers with the workspace search semantics; the
// caller ignores database failures exactly like the original code did.
func (u *customerUsecase) ListForWorkspace(ctx context.Context, q domainCustomer.WorkspaceQuery) ([]domainCustomer.Customer, int64, error) {
	return u.wsRepo.FindAllForWorkspace(ctx, q)
}

// CreateForWorkspace creates a customer from the workspace request body.
func (u *customerUsecase) CreateForWorkspace(ctx context.Context, in WorkspaceCreateInput) (*domainCustomer.Customer, error) {
	c := &domainCustomer.Customer{
		Code:          fmt.Sprintf("CUST-%s", time.Now().Format("20060102150405")),
		Name:          in.Name,
		ContactPerson: in.ContactPerson,
		Email:         in.Email,
		Phone:         in.Phone,
		TaxID:         in.TaxID,
		Address:       in.Address,
		Logo:          in.Logo,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := u.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// SetStatusForWorkspace loads a customer, applies the new status and saves.
// Any lookup error (including parse/record-not-found and real db failures)
// maps to the historical 404; a failed save maps to the historical 500.
func (u *customerUsecase) SetStatusForWorkspace(ctx context.Context, id uint, status string) (*domainCustomer.Customer, error) {
	c, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, appErrors.NewAppError("CUSTOMER_NOT_FOUND", "Customer not found", 404)
	}
	if c == nil {
		return nil, appErrors.NewAppError("CUSTOMER_NOT_FOUND", "Customer not found", 404)
	}

	c.Status = status
	c.UpdatedAt = time.Now()
	if err := u.repo.Update(ctx, c); err != nil {
		return nil, appErrors.NewAppError("CUSTOMER_UPDATE_FAILED", "Failed to update customer status", 500)
	}
	return c, nil
}
