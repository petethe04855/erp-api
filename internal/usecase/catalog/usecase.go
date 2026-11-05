package catalog

import (
	"context"

	domainCatalog "chawy-erp-api/internal/domain/catalog"
	domainSKU "chawy-erp-api/internal/domain/sku"
	pkgDatabase "chawy-erp-api/pkg/database"
)

// Usecase holds the business rules for the workspace product catalog.
// The handler stays a thin parse-delegate: request validation messages and
// database failure responses originate here, and every one of them keeps the
// exact text the inline handler code used to respond with.
type Usecase interface {
	List(ctx context.Context, q domainCatalog.ProductQuery) ([]ProductRecord, int64, error)
	GetByCode(ctx context.Context, code string) (*ProductRecord, error)
	FindByID(ctx context.Context, id string) (*domainSKU.SKU, error)
	Create(ctx context.Context, input CreateInput) (*ProductRecord, error)
	UpdateByID(ctx context.Context, id string, input UpdateInput) (*domainSKU.SKU, error)
	Update(ctx context.Context, code string, input UpdateInput) (*ProductRecord, error)
	SetStatusByID(ctx context.Context, id string, status string) (*domainSKU.SKU, error)
	SetStatus(ctx context.Context, code string, status string) (*domainSKU.SKU, error)
	SetReorderByID(ctx context.Context, id string, reorder int) (*domainSKU.SKU, error)
	SetReorder(ctx context.Context, code string, reorder int) (*domainSKU.SKU, error)
	DeleteByID(ctx context.Context, id string) error
	Delete(ctx context.Context, code string) error
	Resolve(ctx context.Context, codes []string) ([]ResolvedSKU, error)
	Accessories(ctx context.Context, sku string) ([]SKUAccessoryRecord, error)
	Import(ctx context.Context, totalRows int, lines []ImportLine) *ImportSummary
}

type usecase struct {
	repo   domainCatalog.Repository
	stats  ReceiptStatsSource
	txMgr  pkgDatabase.TxManager
}

func NewCatalogUsecase(repo domainCatalog.Repository, stats ReceiptStatsSource, txMgr pkgDatabase.TxManager) Usecase {
	return &usecase{repo: repo, stats: stats, txMgr: txMgr}
}
