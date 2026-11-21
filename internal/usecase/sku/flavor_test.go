package sku_test

import (
	"context"
	"errors"
	"testing"

	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseSKU "chawy-erp-api/internal/usecase/sku"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlavorOnUpdate(t *testing.T) {
	repo := &fakeSKURepo{item: &domainSKU.SKU{ID: 1, SKU: "CHK-01", Name: "ขนมไก่", Flavor: ""}}
	uc := usecaseSKU.NewSKUUsecase(repo)
	ctx := context.Background()

	chicken := " Chicken "
	got, err := uc.Update(ctx, 1, usecaseSKU.UpdateInput{Flavor: &chicken})
	require.NoError(t, err)
	assert.Equal(t, domainSKU.FlavorChicken, got.Flavor, "normalised to lower case")

	name := "ขนมไก่ ใหม่"
	got, err = uc.Update(ctx, 1, usecaseSKU.UpdateInput{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, domainSKU.FlavorChicken, got.Flavor, "omitting flavor keeps it")

	bad := "beef"
	_, err = uc.Update(ctx, 1, usecaseSKU.UpdateInput{Flavor: &bad})
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "INVALID_FLAVOR", appErr.Code)

	empty := ""
	got, err = uc.Update(ctx, 1, usecaseSKU.UpdateInput{Flavor: &empty})
	require.NoError(t, err)
	assert.Empty(t, got.Flavor, "empty clears the flavor")
}

func TestFlavorOnCreate(t *testing.T) {
	uc := usecaseSKU.NewSKUUsecase(&fakeSKURepo{})
	_, err := uc.Create(context.Background(), usecaseSKU.CreateInput{SKU: "X-1", Name: "X", Flavor: "beef"})
	var appErr *appErrors.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "INVALID_FLAVOR", appErr.Code)

	got, err := uc.Create(context.Background(), usecaseSKU.CreateInput{SKU: "T-1", Name: "ทูน่า", Flavor: "TUNA"})
	require.NoError(t, err)
	assert.Equal(t, domainSKU.FlavorTuna, got.Flavor)
}
