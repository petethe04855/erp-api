package handlers

import (
	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"strings"
	"time"
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

	if prod.SKU == "" || strings.TrimSpace(prod.Name) == "" || prod.RetailPrice <= 0 || prod.Stock < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "SKU, product name, and selling price greater than zero are required"})
	}

	// Phase 1 treats every newly-created SKU as physical finished goods.
	// Legacy material/BOM records remain readable, but cannot be created here.
	prod.Type = "Finished Product"
	prod.IsBundle = false
	prod.BomID = nil
	prod.Price = prod.RetailPrice

	// Verify if product already exists
	var count int64
	database.DB.Model(&models.Product{}).Where("sku = ?", prod.SKU).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Product with this SKU already exists"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}
	initialStock := prod.Stock
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&prod).Error; err != nil {
			return err
		}
		if initialStock == 0 {
			return nil
		}

		today := time.Now().Format("2006-01-02")
		lotName := "OPENING-" + prod.SKU
		lot := models.StockLot{
			Code: "LOT-" + lotName, ProductID: prod.ID, SKU: prod.SKU, Lot: lotName,
			Qty: initialStock, RemainingQty: initialStock, LandedUnitCost: 0,
			ReceivedDate: today, GrRef: "OPENING_BALANCE",
		}
		if err := tx.Create(&lot).Error; err != nil {
			return err
		}
		movement := models.StockMovement{
			Code:      fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), prod.SKU),
			ProductID: prod.ID, SKU: prod.SKU, Type: "IN", Qty: initialStock,
			RefDoc: prod.SKU, RefDocType: "opening_stock", Date: today,
			Note: "Opening Stock from SKU creation", ChangedBy: username.(string),
		}
		return tx.Create(&movement).Error
	})
	if err != nil {
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

	requestedSKU, _ := updateData["newSku"].(string)
	if requestedSKU == "" {
		requestedSKU, _ = updateData["new_sku"].(string)
	}
	requestedSKU = strings.ToUpper(strings.TrimSpace(requestedSKU))
	targetStock := prod.Stock
	hasStockUpdate := false
	if rawStock, ok := updateData["stock"]; ok {
		stockNumber, ok := rawStock.(float64)
		if !ok || stockNumber < 0 || stockNumber != float64(int(stockNumber)) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "stock must be a non-negative whole number"})
		}
		targetStock = int(stockNumber)
		hasStockUpdate = true
		if targetStock < prod.ReservedQty {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "stock cannot be lower than reserved quantity"})
		}
	}
	if requestedSKU != "" && requestedSKU != prod.SKU {
		var duplicate int64
		if err := database.DB.Model(&models.Product{}).Where("sku = ?", requestedSKU).Count(&duplicate).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if duplicate > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Product with this SKU already exists"})
		}

		var stockActivity int64
		if err := database.DB.Model(&models.StockLot{}).Where("sku = ?", prod.SKU).Count(&stockActivity).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		var movementActivity int64
		if err := database.DB.Model(&models.StockMovement{}).Where("sku = ?", prod.SKU).Count(&movementActivity).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		var salesActivity int64
		if err := database.DB.Model(&models.SalesOrderLine{}).Where("sku = ?", prod.SKU).Count(&salesActivity).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		if prod.Stock != 0 || prod.ReservedQty != 0 || stockActivity > 0 || movementActivity > 0 || salesActivity > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "cannot change SKU after stock or sales activity; create a new SKU instead",
			})
		}
	}

	// Prevent overwriting primary key / SKU from JSON payload
	delete(updateData, "id")
	delete(updateData, "sku")
	delete(updateData, "newSku")
	delete(updateData, "new_sku")
	delete(updateData, "type")
	delete(updateData, "isBundle")
	delete(updateData, "bomId")
	delete(updateData, "stock")

	// JSON uses camelCase while GORM map updates require database column names.
	columnAliases := map[string]string{
		"weightGrams": "weight_grams", "retailPrice": "retail_price",
		"wholesalePrice": "wholesale_price", "baseUnit": "base_unit",
		"reservedQty": "reserved_qty", "isActive": "is_active",
	}
	for jsonKey, columnName := range columnAliases {
		if value, ok := updateData[jsonKey]; ok {
			updateData[columnName] = value
			delete(updateData, jsonKey)
		}
	}
	if retailPrice, ok := updateData["retail_price"]; ok {
		updateData["price"] = retailPrice
	}

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

	if len(updateData) > 0 || requestedSKU != "" || hasStockUpdate {
		if err := database.DB.Transaction(func(tx *gorm.DB) error {
			if requestedSKU != "" && requestedSKU != prod.SKU {
				if err := tx.Model(&prod).Update("sku", requestedSKU).Error; err != nil {
					return err
				}
				prod.SKU = requestedSKU
			}
			if len(updateData) > 0 {
				if err := tx.Model(&prod).Updates(updateData).Error; err != nil {
					return err
				}
			}
			if !hasStockUpdate || targetStock == prod.Stock {
				return nil
			}

			username := c.Locals("name")
			if username == nil {
				username = "System"
			}
			today := time.Now().Format("2006-01-02")
			difference := targetStock - prod.Stock
			if difference < 0 {
				return deductFefoStock(tx, prod.SKU, -difference, prod.SKU, &prod.ID, "stock_adjustment", username.(string))
			}

			lotName := fmt.Sprintf("ADJUST-%s-%d", prod.SKU, time.Now().UnixNano())
			lot := models.StockLot{
				Code: "LOT-" + lotName, ProductID: prod.ID, SKU: prod.SKU, Lot: lotName,
				Qty: difference, RemainingQty: difference, LandedUnitCost: prod.Cost,
				ReceivedDate: today, GrRef: "STOCK_ADJUSTMENT",
			}
			if err := tx.Create(&lot).Error; err != nil {
				return err
			}
			movement := models.StockMovement{
				Code:      fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), prod.SKU),
				ProductID: prod.ID, SKU: prod.SKU, Type: "IN", Qty: difference,
				RefDoc: prod.SKU, RefDocType: "stock_adjustment", RefDocID: &prod.ID,
				Date: today, Note: "Stock adjusted from SKU edit", ChangedBy: username.(string),
			}
			if err := tx.Create(&movement).Error; err != nil {
				return err
			}
			return tx.Model(&prod).Update("stock", targetStock).Error
		}); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// Reload updated product
	database.DB.First(&prod, "sku = ?", prod.SKU)

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
