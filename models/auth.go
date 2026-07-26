package models

import "time"

// AppUser represents a user of the ERP system
type AppUser struct {
	ID           int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Email        string     `json:"email"`
	Firstname    string     `json:"firstname"`
	Lastname     string     `json:"lastname"`
	Role         string     `json:"role"` // owner, sales, warehouse, accountant
	Password     string     `json:"-"`    // Hashed password, not returned in JSON
	IsActive     bool       `gorm:"not null;default:true" json:"isActive"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	EmailWarning string     `gorm:"-" json:"emailWarning,omitempty"`
}
