package sku_test

import (
	"context"
	"testing"

	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseSKU "chawy-erp-api/internal/usecase/sku"

	"github.com/stretchr/testify/assert"
)

type fakeSKURepo struct {
	item *domainSKU.SKU
}

func (f *fakeSKURepo) Create(ctx context.Context, sku *domainSKU.SKU) error { return nil }

func (f *fakeSKURepo) FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	if f.item != nil && f.item.ID == id {
		cp := *f.item
		return &cp, nil
	}
	return nil, nil
}

func (f *fakeSKURepo) FindBySKU(ctx context.Context, code string) (*domainSKU.SKU, error) {
	return nil, nil
}

func (f *fakeSKURepo) FindAll(ctx context.Context, q domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	return nil, 0, nil
}

func (f *fakeSKURepo) Update(ctx context.Context, sku *domainSKU.SKU) error {
	f.item = sku
	return nil
}

func (f *fakeSKURepo) Delete(ctx context.Context, id uint) error { return nil }

func (f *fakeSKURepo) ExistsBySKU(ctx context.Context, code string) (bool, error) { return false, nil }

// FULL-11: updating only the name must keep cost price, bundle flag and image.
func TestUpdate_PartialUpdatePreservesFields(t *testing.T) {
	repo := &fakeSKURepo{item: &domainSKU.SKU{
		ID:        1,
		SKU:       "ABC",
		Name:      "Old Name",
		CostPrice: 80,
		IsBundle:  true,
		Image:     "logo.png",
		Price:     120,
	}}
	uc := usecaseSKU.NewSKUUsecase(repo)

	newName := "New Name"
	updated, err := uc.Update(context.Background(), 1, usecaseSKU.UpdateInput{
		Name: &newName,
		// everything else omitted — old code zeroed CostPrice and IsBundle
	})
	assert.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, 80.0, updated.CostPrice)
	assert.True(t, updated.IsBundle)
	assert.Equal(t, "logo.png", updated.Image)
	assert.Equal(t, 120.0, updated.Price)
}

// Explicit zero-cost update is still allowed when the caller sends it.
func TestUpdate_ExplicitCostUpdate(t *testing.T) {
	repo := &fakeSKURepo{item: &domainSKU.SKU{
		ID:        1,
		SKU:       "ABC",
		Name:      "Product",
		CostPrice: 80,
	}}
	uc := usecaseSKU.NewSKUUsecase(repo)

	newCost := 45.5
	updated, err := uc.Update(context.Background(), 1, usecaseSKU.UpdateInput{
		CostPrice: &newCost,
	})
	assert.NoError(t, err)
	assert.Equal(t, 45.5, updated.CostPrice)
}
