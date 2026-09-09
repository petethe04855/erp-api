package auth

import (
	"context"
	"time"
)

type User struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Email       string     `json:"email" gorm:"uniqueIndex;not null;size:255"`
	Password    string     `json:"-" gorm:"not null"`
	Firstname   string     `json:"firstname" gorm:"size:100"`
	Lastname    string     `json:"lastname" gorm:"size:100"`
	Name        string     `json:"name" gorm:"size:255"`
	Role        string     `json:"role" gorm:"size:50;default:'sales'"`
	IsActive    bool       `json:"isActive" gorm:"not null;default:true"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Repository interface {
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id uint) (*User, error)
	FindAll(ctx context.Context) ([]*User, error)
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, user *User) error
	CreateWithBootstrapRole(ctx context.Context, user *User) (string, error)
	Update(ctx context.Context, user *User) error
	UpdateStatus(ctx context.Context, id uint, isActive bool) error
	Delete(ctx context.Context, id uint) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
