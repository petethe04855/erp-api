package postgres

import (
	"context"

	domainPermission "chawy-erp-api/internal/domain/permission"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type permissionRepository struct {
	db *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) domainPermission.Repository {
	return &permissionRepository{db: db}
}

func (r *permissionRepository) ListAll(ctx context.Context) ([]domainPermission.RolePermission, error) {
	var rows []domainPermission.RolePermission
	if err := r.db.WithContext(ctx).Order("role, page_key").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *permissionRepository) SaveRole(ctx context.Context, role string, rows []domainPermission.RolePermission) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range rows {
			rows[i].Role = role
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "role"}, {Name: "page_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"can_view", "can_create", "can_edit", "can_delete", "updated_by", "updated_at"}),
		}).Create(&rows).Error
	})
}
