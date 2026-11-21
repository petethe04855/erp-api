// Package crmintegration holds what the CRM backoffice (backoffice-api) reads
// from and asks of the ERP over /api/v1/integrations/crm: SKUs, available
// stock and goods issues for reward shipments. The ERP stays the owner of SKUs
// and stock; the CRM never touches these tables directly.
package crmintegration

import (
	"context"
	"time"
)

// Limits.
const (
	MaxStockSKUs      = 200
	MaxIssueItems     = 50
	MaxIdempotencyKey = 100
	DefaultWarehouse  = 1
	// ReferenceType is stamped on stock movements created for the CRM.
	ReferenceType = "CRM_REWARD"
)

// GoodsIssueItem is one line of a CRM goods issue.
type GoodsIssueItem struct {
	SKUID   uint   `json:"skuId"`
	SKUCode string `json:"skuCode"`
	Qty     int    `json:"qty"`
}

// GoodsIssue records a CRM goods issue so a retried request with the same
// idempotency key returns the first result instead of deducting stock again
// (table crm_goods_issues).
type GoodsIssue struct {
	ID             uint             `gorm:"primaryKey"`
	IdempotencyKey string           `gorm:"size:100;uniqueIndex;not null"`
	GIRef          string           `gorm:"column:gi_ref;size:40;uniqueIndex"`
	WarehouseID    uint             `gorm:"not null"`
	Reference      string           `gorm:"size:100"`
	Note           string           `gorm:"size:255"`
	Items          []GoodsIssueItem `gorm:"serializer:json;type:jsonb"`
	CreatedAt      time.Time
}

// TableName keeps the CRM integration tables grouped.
func (GoodsIssue) TableName() string { return "crm_goods_issues" }

// Repository stores goods-issue records.
type Repository interface {
	FindByKey(ctx context.Context, key string) (*GoodsIssue, error)
	Create(ctx context.Context, gi *GoodsIssue) error
	SetRef(ctx context.Context, id uint, ref string) error
}
