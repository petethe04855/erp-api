package sku

import (
	"context"
	"errors"
	"fmt"
	"time"

	domainSKU "chawy-erp-api/internal/domain/sku"
)

var (
	ErrInvalidCostPrice     = errors.New("cost price must be greater than 0")
	ErrInvalidEffectiveDate = errors.New("invalid effective date format, expected YYYY-MM-DD")
)

type CostHistoryUsecase interface {
	AddCostHistory(ctx context.Context, skuCode string, costPrice float64, effectiveFrom string, effectiveTo *string, note string) (*domainSKU.SKUCostHistory, error)
	GetCostHistoryBySKU(ctx context.Context, skuCode string) ([]domainSKU.SKUCostHistory, error)
	GetEffectiveCost(ctx context.Context, skuCode string, date time.Time) (float64, error)
	DeleteCostHistory(ctx context.Context, id uint) error
}

type costHistoryUsecase struct {
	costRepo domainSKU.CostHistoryRepository
	skuRepo  domainSKU.Repository
}

func NewCostHistoryUsecase(costRepo domainSKU.CostHistoryRepository, skuRepo domainSKU.Repository) CostHistoryUsecase {
	return &costHistoryUsecase{
		costRepo: costRepo,
		skuRepo:  skuRepo,
	}
}

func (u *costHistoryUsecase) AddCostHistory(ctx context.Context, skuCode string, costPrice float64, effectiveFrom string, effectiveTo *string, note string) (*domainSKU.SKUCostHistory, error) {
	if costPrice <= 0 {
		return nil, ErrInvalidCostPrice
	}

	fromTime, err := time.Parse("2006-01-02", effectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEffectiveDate, err)
	}

	var toTime *time.Time
	if effectiveTo != nil && *effectiveTo != "" {
		t, err := time.Parse("2006-01-02", *effectiveTo)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidEffectiveDate, err)
		}
		if t.Before(fromTime) {
			return nil, errors.New("effective_to cannot be earlier than effective_from")
		}
		toTime = &t
	}

	record := &domainSKU.SKUCostHistory{
		SKU:           skuCode,
		CostPrice:     costPrice,
		EffectiveFrom: fromTime,
		EffectiveTo:   toTime,
		Note:          note,
	}

	if err := u.costRepo.Create(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

func (u *costHistoryUsecase) GetCostHistoryBySKU(ctx context.Context, skuCode string) ([]domainSKU.SKUCostHistory, error) {
	return u.costRepo.FindBySKU(ctx, skuCode)
}

func (u *costHistoryUsecase) GetEffectiveCost(ctx context.Context, skuCode string, date time.Time) (float64, error) {
	// 1. Try finding in cost history
	cost, found, err := u.costRepo.FindEffectiveCost(ctx, skuCode, date)
	if err != nil {
		return 0, err
	}
	if found {
		return cost, nil
	}

	// 2. Fallback to Master SKU default cost_price
	skuObj, err := u.skuRepo.FindBySKU(ctx, skuCode)
	if err != nil {
		return 0, err
	}
	if skuObj != nil {
		return skuObj.CostPrice, nil
	}

	return 0, nil
}

func (u *costHistoryUsecase) DeleteCostHistory(ctx context.Context, id uint) error {
	return u.costRepo.Delete(ctx, id)
}
