package handlers

import (
	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// POST /api/products
func CreateProduct(c *fiber.Ctx) error {
	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var prod models.Product
	if err := c.BodyParser(&prod); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Default IsActive to true if not explicitly passed in payload
	if _, exists := body["isActive"]; !exists {
		prod.IsActive = true
	}

	if prod.SKU == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "SKU is required"})
	}

	// Verify if product already exists
	var count int64
	database.DB.Model(&models.Product{}).Where("sku = ?", prod.SKU).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Product with this SKU already exists"})
	}

	if err := database.DB.Create(&prod).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(prod)
}

// PUT /api/products/:sku
func UpdateProduct(c *fiber.Ctx) error {
	sku := c.Params("sku")
	var prod models.Product
	if err := database.DB.First(&prod, "sku = ?", sku).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	var updateData map[string]interface{}
	if err := c.BodyParser(&updateData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Prevent overwriting primary key / SKU from JSON payload
	delete(updateData, "id")
	delete(updateData, "sku")

	// Quantities in Product, StockLot, and StockMovement are stored in the
	// product's base unit. Changing that unit after inventory activity would
	// relabel historical numbers without converting them and corrupt balances.
	requestedBaseUnit, hasBaseUnit := updateData["baseUnit"].(string)
	if !hasBaseUnit {
		requestedBaseUnit, hasBaseUnit = updateData["base_unit"].(string)
	}
	if hasBaseUnit && requestedBaseUnit != "" && requestedBaseUnit != prod.BaseUnit {
		var lotCount int64
		if err := database.DB.Model(&models.StockLot{}).
			Where("sku = ?", sku).
			Count(&lotCount).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		var movementCount int64
		if err := database.DB.Model(&models.StockMovement{}).
			Where("sku = ?", sku).
			Count(&movementCount).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if prod.Stock != 0 || prod.ReservedQty != 0 || lotCount > 0 || movementCount > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "cannot change base unit after stock activity; create a new SKU or convert inventory through Stock Adjustment",
			})
		}
	}

	if len(updateData) > 0 {
		if err := database.DB.Model(&prod).Updates(updateData).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// Reload updated product
	database.DB.First(&prod, "sku = ?", sku)

	return c.JSON(prod)
}

// DELETE /api/products/:sku
func DeleteProduct(c *fiber.Ctx) error {
	sku := c.Params("sku")
	var prod models.Product
	if err := database.DB.First(&prod, "sku = ?", sku).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Delete bundle components associated with this product
		if err := tx.Where("bundle_sku = ? OR component_sku = ?", sku, sku).Delete(&models.BundleComponent{}).Error; err != nil {
			return err
		}
		// Delete product
		if err := tx.Delete(&prod).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true})
}

type SetBundleComponentsRequest struct {
	BundleSku  string `json:"bundleSku"`
	Components []struct {
		ComponentSku     string  `json:"componentSku"`
		Qty              float64 `json:"qty"`
		Unit             string  `json:"unit"`
		ComponentType    string  `json:"componentType"`
		UnitCostOverride float64 `json:"unitCostOverride"`
	} `json:"components"`
}

// POST /api/bundle-components
func SetBundleComponents(c *fiber.Ctx) error {
	var req SetBundleComponentsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var bundleComponents []models.BundleComponent
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Delete existing components for this bundle
		if err := tx.Where("bundle_sku = ?", req.BundleSku).Delete(&models.BundleComponent{}).Error; err != nil {
			return err
		}

		// Insert new components
		for _, comp := range req.Components {
			if comp.Qty <= 0 {
				return fmt.Errorf("component qty must be greater than zero")
			}
			if comp.ComponentType != "expense" && comp.ComponentSku == "" {
				return fmt.Errorf("component SKU is required")
			}
			bc := models.BundleComponent{
				BundleSku:        req.BundleSku,
				ComponentSku:     comp.ComponentSku,
				Qty:              comp.Qty,
				Unit:             comp.Unit,
				ComponentType:    comp.ComponentType,
				UnitCostOverride: comp.UnitCostOverride,
			}
			if err := tx.Create(&bc).Error; err != nil {
				return err
			}
			bundleComponents = append(bundleComponents, bc)
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var bundle models.Product
	if err := database.DB.First(&bundle, "sku = ?", req.BundleSku).Error; err == nil {
		cost, err := calculateBOMCost(database.DB, bundleComponents)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		bundle.Cost = cost
		database.DB.Save(&bundle)
	}

	return c.JSON(bundleComponents)
}

func calculateBOMCost(db *gorm.DB, components []models.BundleComponent) (float64, error) {
	if len(components) == 0 {
		return 0, nil
	}
	total := 0.0
	parentSku := components[0].BundleSku
	for _, comp := range components {
		if comp.ComponentType == "expense" {
			total += comp.Qty * comp.UnitCostOverride
			continue
		}
		var product models.Product
		if err := db.First(&product, "sku = ?", comp.ComponentSku).Error; err != nil {
			return 0, fmt.Errorf("component product %s not found", comp.ComponentSku)
		}
		unitCost := product.Cost
		if unitCost == 0 && comp.UnitCostOverride > 0 {
			unitCost = comp.UnitCostOverride
		}
		yield := comp.YieldFactor
		if yield <= 0 {
			yield = 1
		}
		qty := convertBOMQty(grossBOMQty(comp.Qty/yield, comp.ScrapRate), comp.Unit, product.BaseUnit)
		total += qty * unitCost
	}

	batchSize := componentQtyBatchSize(parentSku)
	if batchSize <= 0 {
		batchSize = 1
	}
	return total / batchSize, nil
}
