package customer

import (
	"context"
	"fmt"
	"time"

	domainCustomer "chawy-erp-api/internal/domain/customer"
	appErrors "chawy-erp-api/pkg/errors"
)

type CreateInput struct {
	Code          string
	Name          string
	ContactPerson string
	Phone         string
	Email         string
	Address       string
	TaxID         string
	Channel       string
}

type UpdateInput struct {
	Name          string
	ContactPerson string
	Phone         string
	Email         string
	Address       string
	TaxID         string
	Channel       string
	Status        string
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*domainCustomer.Customer, error)
	GetByID(ctx context.Context, id uint) (*domainCustomer.Customer, error)
	List(ctx context.Context, query domainCustomer.Query) ([]domainCustomer.Customer, int64, error)
	Update(ctx context.Context, id uint, input UpdateInput) (*domainCustomer.Customer, error)
	Delete(ctx context.Context, id uint) error
}

type customerUsecase struct {
	repo domainCustomer.Repository
}

func NewCustomerUsecase(repo domainCustomer.Repository) Usecase {
	return &customerUsecase{repo: repo}
}

func (u *customerUsecase) Create(ctx context.Context, in CreateInput) (*domainCustomer.Customer, error) {
	code := in.Code
	if code == "" {
		code = fmt.Sprintf("CUST-%d", time.Now().UnixNano()%1000000)
	}

	c := &domainCustomer.Customer{
		Code:          code,
		Name:          in.Name,
		ContactPerson: in.ContactPerson,
		Phone:         in.Phone,
		Email:         in.Email,
		Address:       in.Address,
		TaxID:         in.TaxID,
		Channel:       in.Channel,
		Status:        "active",
	}

	if err := u.repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (u *customerUsecase) GetByID(ctx context.Context, id uint) (*domainCustomer.Customer, error) {
	c, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, appErrors.ErrNotFound
	}
	return c, nil
}

func (u *customerUsecase) List(ctx context.Context, query domainCustomer.Query) ([]domainCustomer.Customer, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	return u.repo.FindAll(ctx, query)
}

func (u *customerUsecase) Update(ctx context.Context, id uint, in UpdateInput) (*domainCustomer.Customer, error) {
	c, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, appErrors.ErrNotFound
	}

	if in.Name != "" {
		c.Name = in.Name
	}
	if in.ContactPerson != "" {
		c.ContactPerson = in.ContactPerson
	}
	if in.Phone != "" {
		c.Phone = in.Phone
	}
	if in.Email != "" {
		c.Email = in.Email
	}
	if in.Address != "" {
		c.Address = in.Address
	}
	if in.TaxID != "" {
		c.TaxID = in.TaxID
	}
	if in.Channel != "" {
		c.Channel = in.Channel
	}
	if in.Status != "" {
		c.Status = in.Status
	}

	if err := u.repo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (u *customerUsecase) Delete(ctx context.Context, id uint) error {
	c, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if c == nil {
		return appErrors.ErrNotFound
	}
	return u.repo.Delete(ctx, id)
}
