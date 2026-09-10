package dto

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
	Channel          string `json:"channel"`
	Status           string `json:"status"`
}
