package dto

import (
	"time"

	domainCustomer "chawy-erp-api/internal/domain/customer"
)

type CreateCustomerRequest struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	ContactPerson    string `json:"contact_person"`
	ContactPersonAlt string `json:"contactPerson"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	Address          string `json:"address"`
	TaxID            string `json:"tax_id"`
	TaxIDAlt         string `json:"taxId"`
	Logo             string `json:"logo"`
	Channel          string `json:"channel"`
}

type UpdateCustomerRequest struct {
	Name             string `json:"name"`
	ContactPerson    string `json:"contact_person"`
	ContactPersonAlt string `json:"contactPerson"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	Address          string `json:"address"`
	TaxID            string `json:"tax_id"`
	TaxIDAlt         string `json:"taxId"`
	Logo             string `json:"logo"`
	Channel          string `json:"channel"`
	Status           string `json:"status"`
}

// CustomerResponse is the API contract for customer payloads. It replaces the
// raw domain entity (snake_case JSON, e.g. contact_person/tax_id) with a
// stable camelCase contract the frontend maps to directly.
type CustomerResponse struct {
	ID            uint   `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	ContactPerson string `json:"contactPerson"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Address       string `json:"address"`
	TaxID         string `json:"taxId"`
	Logo          string `json:"logo"`
	Channel       string `json:"channel"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
}

// NewCustomerResponse maps a domain customer to the API contract.
func NewCustomerResponse(c *domainCustomer.Customer) CustomerResponse {
	resp := CustomerResponse{
		ID:            c.ID,
		Code:          c.Code,
		Name:          c.Name,
		ContactPerson: c.ContactPerson,
		Phone:         c.Phone,
		Email:         c.Email,
		Address:       c.Address,
		TaxID:         c.TaxID,
		Logo:          c.Logo,
		Channel:       c.Channel,
		Status:        c.Status,
	}
	if !c.CreatedAt.IsZero() {
		resp.CreatedAt = c.CreatedAt.Format(time.RFC3339)
	}
	if !c.UpdatedAt.IsZero() {
		resp.UpdatedAt = c.UpdatedAt.Format(time.RFC3339)
	}
	return resp
}

// NewCustomerResponses maps a slice of domain customers.
func NewCustomerResponses(items []domainCustomer.Customer) []CustomerResponse {
	out := make([]CustomerResponse, len(items))
	for i := range items {
		out[i] = NewCustomerResponse(&items[i])
	}
	return out
}
