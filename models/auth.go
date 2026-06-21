package models

import "time"

// AppUser represents a user of the ERP system
type AppUser struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	Name        string     `json:"name"`
	Role        string     `json:"role"` // owner, sales, warehouse, accountant
	Password    string     `json:"-"`    // Hashed password, not returned in JSON
	IsActive    bool       `gorm:"not null;default:true" json:"isActive"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
}
