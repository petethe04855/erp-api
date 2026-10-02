package sku

import (
	"context"
	"time"
)

type SKUCostHistory struct {
	ID            uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	SKU           string     `json:"sku" gorm:"index;not null;size:100"`
	CostPrice     float64    `json:"cost_price" gorm:"type:numeric(12,2);not null"`
	EffectiveFrom time.Time  `json:"effective_from" gorm:"type:date;not null;index"`
	EffectiveTo   *time.Time `json:"effective_to" gorm:"type:date;index"`
	Note          string     `json:"note" gorm:"size:255"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (SKUCostHistory) TableName() string { return "sku_cost_histories" }

type CostHistoryRepository interface {
	Create(ctx context.Context, item *SKUCostHistory) error
	FindBySKU(ctx context.Context, sku string) ([]SKUCostHistory, error)
	FindEffectiveCost(ctx context.Context, sku string, date time.Time) (float64, bool, error)
	Delete(ctx context.Context, id uint) error
}
