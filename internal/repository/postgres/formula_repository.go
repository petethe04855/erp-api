package postgres

import (
	"context"
	"strings"

	domainFormula "chawy-erp-api/internal/domain/formula"

	"gorm.io/gorm"
)

type FormulaRepository struct {
	db *gorm.DB
}

func NewFormulaRepository(db *gorm.DB) *FormulaRepository {
	return &FormulaRepository{db: db}
}

func (r *FormulaRepository) Create(ctx context.Context, formula *domainFormula.InventoryFormula) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(formula).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *FormulaRepository) Update(ctx context.Context, formula *domainFormula.InventoryFormula) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update header
		updates := map[string]interface{}{
			"name":        formula.Name,
			"description": formula.Description,
			"is_active":   formula.IsActive,
			"updated_at":  formula.UpdatedAt,
		}
		if formula.Image != "" || formula.Image == "" {
			updates["image"] = formula.Image
		}
		if err := tx.Model(&domainFormula.InventoryFormula{}).
			Where("code = ?", formula.Code).
			Updates(updates).Error; err != nil {
			return err
		}

		// Delete old items
		if err := tx.Where("formula_code = ?", formula.Code).Delete(&domainFormula.InventoryFormulaItem{}).Error; err != nil {
			return err
		}

		// Insert new items
		if len(formula.Items) > 0 {
			if err := tx.Create(&formula.Items).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *FormulaRepository) Deactivate(ctx context.Context, code string) error {
	return r.ToggleStatus(ctx, code, false)
}

func (r *FormulaRepository) ToggleStatus(ctx context.Context, code string, isActive bool) error {
	return r.db.WithContext(ctx).Model(&domainFormula.InventoryFormula{}).
		Where("UPPER(code) = ?", strings.ToUpper(strings.TrimSpace(code))).
		Update("is_active", isActive).Error
}

func (r *FormulaRepository) FindByCode(ctx context.Context, code string) (*domainFormula.InventoryFormula, error) {
	var f domainFormula.InventoryFormula
	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("UPPER(code) = ?", strings.ToUpper(strings.TrimSpace(code))).
		First(&f).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

func (r *FormulaRepository) FindAll(ctx context.Context, q domainFormula.Query) ([]domainFormula.InventoryFormula, int64, error) {
	var list []domainFormula.InventoryFormula
	var total int64

	db := r.db.WithContext(ctx).Model(&domainFormula.InventoryFormula{})

	if q.Search != "" {
		s := "%" + strings.ToUpper(strings.TrimSpace(q.Search)) + "%"
		db = db.Where("UPPER(code) LIKE ? OR UPPER(name) LIKE ?", s, s)
	}

	if q.IsActive != nil {
		db = db.Where("is_active = ?", *q.IsActive)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}
	offset := (q.Page - 1) * q.Limit

	if err := db.Preload("Items").
		Offset(offset).
		Limit(q.Limit).
		Order("code ASC").
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

func (r *FormulaRepository) ExistsByCode(ctx context.Context, code string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domainFormula.InventoryFormula{}).
		Where("UPPER(code) = ?", strings.ToUpper(strings.TrimSpace(code))).
		Count(&count).Error
	return count > 0, err
}
