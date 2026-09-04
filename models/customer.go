package models

// Customer represents a client company or buyer
type Customer struct {
	ID            string `gorm:"primaryKey" json:"id"` // e.g. "CUST-0001"
	Name          string `gorm:"index;not null" json:"name"`
	TaxID         string `json:"taxId"`
	Branch        string `json:"branch"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Website       string `json:"website"`
	ContactPerson string `json:"contactPerson"`
	Address       string `json:"address"`
	LogoURL       string `json:"logoUrl"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}
