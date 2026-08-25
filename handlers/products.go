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
	var bundleInput struct {
		Components []BundleComponentRequest `json:"components"`
	}
	if err := c.BodyParser(&bundleInput); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Default IsActive to true if not explicitly passed in payload
	if _, exists := body["isActive"]; !exists {
		prod.IsActive = true
	}

	if prod.SKU == "" || strings.TrimSpace(prod.Name) == "" || prod.RetailPrice <= 0 || prod.Stock < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "SKU, product name, and selling price greater than zero are required"})
	}

	if prod.IsBundle || prod.Type == "Bundle" {
		prod.Type = "Bundle"
		prod.IsBundle = true
		if prod.Stock != 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bundle stock is calculated from its components and must be zero"})
		}
		if len(bundleInput.Components) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bundle requires at least one component"})
		}
	} else {
		prod.Type = "Finished Product"
		prod.IsBundle = false
		bundleInput.Components = nil
	}
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
		if prod.IsBundle {
			components, err := replaceBundleComponents(tx, prod, bundleInput.Components)
			if err != nil {
				return err
			}
			cost, err := calculateBOMCost(tx, components)
			if err != nil {
				return err
			}
			prod.Cost = cost
			return tx.Model(&prod).Update("cost", cost).Error
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
	var bundleInput struct {
		Components []BundleComponentRequest `json:"components"`
	}
	if err := c.BodyParser(&bundleInput); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	_, componentsProvided := updateData["components"]
	targetIsBundle := prod.IsBundle
	if value, ok := updateData["isBundle"].(bool); ok {
		targetIsBundle = value
	}
	if value, ok := updateData["type"].(string); ok {
		targetIsBundle = value == "Bundle"
	}
	if targetIsBundle && componentsProvided && len(bundleInput.Components) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "bundle requires at least one component"})
	}
	if !prod.IsBundle && targetIsBundle && (prod.Stock > 0 || prod.ReservedQty > 0) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot convert a stocked product to a bundle; adjust stock to zero first"})
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
	delete(updateData, "components")
	delete(updateData, "bomId")
	delete(updateData, "stock")
	if targetIsBundle {
		updateData["type"] = "Bundle"
		updateData["is_bundle"] = true
	} else {
		updateData["type"] = "Finished Product"
		updateData["is_bundle"] = false
	}

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
			originalSKU := prod.SKU
			if requestedSKU != "" && requestedSKU != prod.SKU {
				if err := tx.Model(&prod).Update("sku", requestedSKU).Error; err != nil {
					return err
				}
				prod.SKU = requestedSKU
				if err := tx.Model(&models.BundleComponent{}).Where("bundle_sku = ?", originalSKU).Update("bundle_sku", requestedSKU).Error; err != nil {
					return err
				}
				if err := tx.Model(&models.BundleComponent{}).Where("component_sku = ?", originalSKU).Update("component_sku", requestedSKU).Error; err != nil {
					return err
				}
			}
			if len(updateData) > 0 {
				if err := tx.Model(&prod).Updates(updateData).Error; err != nil {
					return err
				}
			}
			if hasStockUpdate && targetStock != prod.Stock {
				username := c.Locals("name")
				if username == nil {
					username = "System"
				}
				today := time.Now().Format("2006-01-02")
				difference := targetStock - prod.Stock
				if difference < 0 {
					if err := deductFefoStock(tx, prod.SKU, -difference, prod.SKU, &prod.ID, "stock_adjustment", username.(string)); err != nil {
						return err
					}
				} else {
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
						Code: fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), prod.SKU), ProductID: prod.ID,
						SKU: prod.SKU, Type: "IN", Qty: difference, RefDoc: prod.SKU,
						RefDocType: "stock_adjustment", RefDocID: &prod.ID, Date: today,
						Note: "Stock adjusted from SKU edit", ChangedBy: username.(string),
					}
					if err := tx.Create(&movement).Error; err != nil {
						return err
					}
					if err := tx.Model(&prod).Update("stock", targetStock).Error; err != nil {
						return err
					}
				}
			}

			if err := tx.First(&prod, prod.ID).Error; err != nil {
				return err
			}
			if componentsProvided {
				components, err := replaceBundleComponents(tx, prod, bundleInput.Components)
				if err != nil {
					return err
				}
				if targetIsBundle {
					cost, err := calculateBOMCost(tx, components)
					if err != nil {
						return err
					}
					if err := tx.Model(&prod).Update("cost", cost).Error; err != nil {
						return err
					}
				}
			} else if !targetIsBundle {
				if err := tx.Where("bundle_sku = ?", prod.SKU).Delete(&models.BundleComponent{}).Error; err != nil {
					return err
				}
			}
			return nil
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

type BundleComponentRequest struct {
	ComponentSku     string  `json:"componentSku"`
	Qty              float64 `json:"qty"`
	Unit             string  `json:"unit"`
	ComponentType    string  `json:"componentType"`
	UnitCostOverride float64 `json:"unitCostOverride"`
}

type SetBundleComponentsRequest struct {
	BundleSku  string                   `json:"bundleSku"`
	Components []BundleComponentRequest `json:"components"`
}

// POST /api/bundle-components
func SetBundleComponents(c *fiber.Ctx) error {
	var req SetBundleComponentsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var bundle models.Product
	if err := database.DB.First(&bundle, "sku = ?", strings.ToUpper(strings.TrimSpace(req.BundleSku))).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "bundle product not found"})
	}
	if !bundle.IsBundle {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "selected SKU is not a bundle"})
	}

	var bundleComponents []models.BundleComponent
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		bundleComponents, err = replaceBundleComponents(tx, bundle, req.Components)
		return err
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	cost, err := calculateBOMCost(database.DB, bundleComponents)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	bundle.Cost = cost
	database.DB.Save(&bundle)

	return c.JSON(bundleComponents)
}

func replaceBundleComponents(tx *gorm.DB, bundle models.Product, input []BundleComponentRequest) ([]models.BundleComponent, error) {
	if !bundle.IsBundle {
		if len(input) > 0 {
			return nil, fmt.Errorf("selected SKU is not a bundle")
		}
		if err := tx.Where("bundle_sku = ?", bundle.SKU).Delete(&models.BundleComponent{}).Error; err != nil {
			return nil, err
		}
		return []models.BundleComponent{}, nil
	}
	if len(input) == 0 {
		return nil, fmt.Errorf("bundle requires at least one component")
	}
	if err := tx.Where("bundle_sku = ?", bundle.SKU).Delete(&models.BundleComponent{}).Error; err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(input))
	components := make([]models.BundleComponent, 0, len(input))
	for _, item := range input {
		sku := strings.ToUpper(strings.TrimSpace(item.ComponentSku))
		if sku == "" || item.Qty <= 0 {
			return nil, fmt.Errorf("component SKU and quantity greater than zero are required")
		}
		if sku == bundle.SKU {
			return nil, fmt.Errorf("bundle cannot contain itself")
		}
		if seen[sku] {
			return nil, fmt.Errorf("component SKU %s is duplicated", sku)
		}
		seen[sku] = true

		var product models.Product
		if err := tx.First(&product, "sku = ?", sku).Error; err != nil {
			return nil, fmt.Errorf("component product %s not found", sku)
		}
		if product.IsBundle {
			return nil, fmt.Errorf("nested bundle %s is not supported", sku)
		}
		if !isSellableProduct(product) {
			return nil, fmt.Errorf("component product %s must be a finished product", sku)
		}
		unit := item.Unit
		if unit == "" {
			unit = "piece"
		}
		componentType := item.ComponentType
		if componentType == "" {
			componentType = "material"
		}
		component := models.BundleComponent{
			BundleProductID: bundle.ID, BundleSku: bundle.SKU,
			ComponentProductID: product.ID, ComponentSku: product.SKU,
			ComponentName: product.Name, Qty: item.Qty, Unit: unit,
			ComponentType: componentType, UnitCostOverride: item.UnitCostOverride,
			YieldFactor: 1,
		}
		if err := tx.Create(&component).Error; err != nil {
			return nil, err
		}
		components = append(components, component)
	}
	return components, nil
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
