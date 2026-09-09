package customer

import (
	"context"
	"time"
)

type Customer struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Code          string    `json:"code" gorm:"uniqueIndex;size:50"`
	Name          string    `json:"name" gorm:"not null;size:255"`
	ContactPerson string    `json:"contact_person" gorm:"size:255"`
	Phone         string    `json:"phone" gorm:"size:50"`
	Email         string    `json:"email" gorm:"size:255"`
	Address       string    `json:"address" gorm:"type:text"`
	TaxID         string    `json:"tax_id" gorm:"size:50"`
	Channel       string    `json:"channel" gorm:"size:50;default:'direct'"` // tiktok, shopee, direct, etc.
	Status        string    `json:"status" gorm:"size:50;default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Query struct {
	Search  string
	Channel string
	Status  string
	Page    int
	Limit   int
}

type Repository interface {
	Create(ctx context.Context, customer *Customer) error
	FindByID(ctx context.Context, id uint) (*Customer, error)
	FindByCode(ctx context.Context, code string) (*Customer, error)
	FindAll(ctx context.Context, query Query) ([]Customer, int64, error)
	Update(ctx context.Context, customer *Customer) error
	Delete(ctx context.Context, id uint) error
}
