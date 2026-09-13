package formula

import (
	"context"
	"time"
)

type InventoryFormula struct {
	ID          uint                   `json:"id" gorm:"primaryKey"`
	Code        string                 `json:"code" gorm:"size:100;uniqueIndex;not null"`
	Name        string                 `json:"name" gorm:"size:255;not null"`
	Description string                 `json:"description" gorm:"size:500"`
	IsActive    bool                   `json:"isActive" gorm:"column:is_active;default:true"`
	Items       []InventoryFormulaItem `json:"items,omitempty" gorm:"foreignKey:FormulaCode;references:Code"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

type InventoryFormulaItem struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	FormulaCode  string    `json:"formulaCode" gorm:"column:formula_code;size:100;index;not null"`
	ComponentSKU string    `json:"componentSku" gorm:"column:component_sku;size:100;index;not null"`
	Qty          int       `json:"qty" gorm:"not null;default:1"`
	Unit         string    `json:"unit" gorm:"size:50"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Query struct {
	Search   string
	IsActive *bool
	Page     int
	Limit    int
}

type Repository interface {
	Create(ctx context.Context, formula *InventoryFormula) error
	Update(ctx context.Context, formula *InventoryFormula) error
	Deactivate(ctx context.Context, code string) error
	ToggleStatus(ctx context.Context, code string, isActive bool) error
	FindByCode(ctx context.Context, code string) (*InventoryFormula, error)
	FindAll(ctx context.Context, query Query) ([]InventoryFormula, int64, error)
	ExistsByCode(ctx context.Context, code string) (bool, error)
}
