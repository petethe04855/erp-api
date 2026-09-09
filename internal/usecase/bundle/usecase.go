package bundle

import (
	"context"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	appErrors "chawy-erp-api/pkg/errors"
)

type ComponentInput struct {
	ComponentSKU string `json:"component_sku"`
	Quantity     int    `json:"quantity"`
	Note         string `json:"note"`
}

type Usecase interface {
	SetComponents(ctx context.Context, bundleSKU string, components []ComponentInput) error
	GetComponents(ctx context.Context, bundleSKU string) ([]domainBundle.BundleItem, error)
	ExplodeBundle(ctx context.Context, bundleSKU string, orderQty int, warehouseID uint) ([]domainBundle.ExplodedItem, bool, error)
}

type bundleUsecase struct {
	bundleRepo domainBundle.Repository
	skuRepo    domainSKU.Repository
	stockRepo  domainStock.Repository
}

func NewBundleUsecase(
	bundleRepo domainBundle.Repository,
	skuRepo domainSKU.Repository,
	stockRepo domainStock.Repository,
) Usecase {
	return &bundleUsecase{
		bundleRepo: bundleRepo,
		skuRepo:    skuRepo,
		stockRepo:  stockRepo,
	}
}

func (u *bundleUsecase) SetComponents(ctx context.Context, bundleSKU string, components []ComponentInput) error {
	// Verify that the bundle SKU exists
	bundleItem, err := u.skuRepo.FindBySKU(ctx, bundleSKU)
	if err != nil {
		return err
	}
	if bundleItem == nil {
		return appErrors.ErrSKUNotFound
	}

	items := make([]domainBundle.BundleItem, len(components))
	for i, c := range components {
		if c.Quantity <= 0 {
			c.Quantity = 1
		}
		items[i] = domainBundle.BundleItem{
			BundleSKU:    bundleSKU,
			ComponentSKU: c.ComponentSKU,
			Quantity:     c.Quantity,
			Note:         c.Note,
		}
	}

	// Update is_bundle = true on the SKU
	if !bundleItem.IsBundle {
		bundleItem.IsBundle = true
		_ = u.skuRepo.Update(ctx, bundleItem)
	}

	return u.bundleRepo.SaveItems(ctx, bundleSKU, items)
}

func (u *bundleUsecase) GetComponents(ctx context.Context, bundleSKU string) ([]domainBundle.BundleItem, error) {
	return u.bundleRepo.GetItemsByBundleSKU(ctx, bundleSKU)
}

func (u *bundleUsecase) ExplodeBundle(
	ctx context.Context,
	bundleSKU string,
	orderQty int,
	warehouseID uint,
) ([]domainBundle.ExplodedItem, bool, error) {
	if orderQty <= 0 {
		orderQty = 1
	}
	if warehouseID == 0 {
		warehouseID = 1
	}

	items, err := u.bundleRepo.GetItemsByBundleSKU(ctx, bundleSKU)
	if err != nil {
		return nil, false, err
	}

	var exploded []domainBundle.ExplodedItem
	allInStock := true

	for _, item := range items {
		neededQty := item.Quantity * orderQty

		compSKU, err := u.skuRepo.FindBySKU(ctx, item.ComponentSKU)
		availQty := 0
		if err == nil && compSKU != nil {
			stk, err := u.stockRepo.GetBySKUID(ctx, compSKU.ID, warehouseID)
			if err == nil && stk != nil {
				availQty = stk.AvailableQty
			}
		}

		hasStock := availQty >= neededQty
		if !hasStock {
			allInStock = false
		}

		exploded = append(exploded, domainBundle.ExplodedItem{
			ComponentSKU: item.ComponentSKU,
			Quantity:     neededQty,
			AvailableQty: availQty,
			HasStock:     hasStock,
		})
	}

	return exploded, allInStock, nil
}
