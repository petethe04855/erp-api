package postgres

import (
	"context"
	"errors"

	domainCRMIntegration "chawy-erp-api/internal/domain/crmintegration"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
)

// CRMIntegrationRepository stores CRM goods-issue records.
type CRMIntegrationRepository struct {
	db *gorm.DB
}

// NewCRMIntegrationRepository builds the repository.
func NewCRMIntegrationRepository(db *gorm.DB) domainCRMIntegration.Repository {
	return &CRMIntegrationRepository{db: db}
}

func (r *CRMIntegrationRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

// FindByKey returns nil, nil when the key was never used.
func (r *CRMIntegrationRepository) FindByKey(ctx context.Context, key string) (*domainCRMIntegration.GoodsIssue, error) {
	var gi domainCRMIntegration.GoodsIssue
	err := r.getDB(ctx).Where("idempotency_key = ?", key).First(&gi).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &gi, nil
}

func (r *CRMIntegrationRepository) Create(ctx context.Context, gi *domainCRMIntegration.GoodsIssue) error {
	return r.getDB(ctx).Create(gi).Error
}

func (r *CRMIntegrationRepository) SetRef(ctx context.Context, id uint, ref string) error {
	return r.getDB(ctx).Model(&domainCRMIntegration.GoodsIssue{}).Where("id = ?", id).Update("gi_ref", ref).Error
}
