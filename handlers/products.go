package handlers

import (
	"chawy-erp-api/database"
	"chawy-erp-api/models"
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
		ComponentSku string `json:"componentSku"`
		Qty          int    `json:"qty"`
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
			bc := models.BundleComponent{
				BundleSku:    req.BundleSku,
				ComponentSku: comp.ComponentSku,
				Qty:          comp.Qty,
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

	return c.JSON(bundleComponents)
}
