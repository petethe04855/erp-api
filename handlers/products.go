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
	var prod models.Product
	if err := c.BodyParser(&prod); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
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

	if err := c.BodyParser(&prod); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Always keep primary key SKU
	prod.SKU = sku

	if err := database.DB.Save(&prod).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

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
	total := 0.0
	for _, comp := range components {
		if comp.ComponentType == "expense" {
			total += comp.Qty * comp.UnitCostOverride
			continue
		}
		var product models.Product
		if err := db.First(&product, "sku = ?", comp.ComponentSku).Error; err != nil {
			return 0, fmt.Errorf("component product %s not found", comp.ComponentSku)
		}
		factor := 1.0
		switch comp.Unit {
		case "g":
			if product.BaseUnit == "kg" {
				factor = 0.001
			}
		case "kg":
			if product.BaseUnit == "g" {
				factor = 1000
			}
		}
		yield := comp.YieldFactor
		if yield <= 0 {
			yield = 1
		}
		total += (comp.Qty / yield) * factor * product.Cost
	}
	return total, nil
}
