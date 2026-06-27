package handlers

import (
	"fmt"
	"math"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type BOMSummary struct {
	SKU              string  `json:"sku"`
	Name             string  `json:"name"`
	ComponentCount   int     `json:"componentCount"`
	Cost             float64 `json:"cost"`
	IsActive         bool    `json:"isActive"`
	BomCode          string  `json:"bomCode"`
	BomName          string  `json:"bomName"`
	BomOutputQty     float64 `json:"bomOutputQty"`
	BomUnit          string  `json:"bomUnit"`
	BomStatus        string  `json:"bomStatus"`
	BomEffectiveDate string  `json:"bomEffectiveDate"`
}

type BOMLineResponse struct {
	SKU                 string  `json:"sku"`
	Name                string  `json:"name"`
	Category            string  `json:"category"`
	Unit                string  `json:"unit"`
	QtyPerUnit          float64 `json:"qtyPerUnit"`
	RequiredQty         float64 `json:"requiredQty"`
	StockQty            float64 `json:"stockQty"`
	Shortage            float64 `json:"shortage"`
	UnitCost            float64 `json:"unitCost"`
	CostPerFinishedUnit float64 `json:"costPerFinishedUnit"`
	PRValue             float64 `json:"prValue"`
	CanCreatePR         bool    `json:"canCreatePr"`
}

type BOMDetailResponse struct {
	SKU              string            `json:"sku"`
	Name             string            `json:"name"`
	ProductionQty    int               `json:"productionQty"`
	ComponentCount   int               `json:"componentCount"`
	PRRequired       int               `json:"prRequired"`
	ReadyItems       int               `json:"readyItems"`
	TotalPRValue     float64           `json:"totalPrValue"`
	TotalCostPerUnit float64           `json:"totalCostPerUnit"`
	BomCode          string            `json:"bomCode"`
	BomName          string            `json:"bomName"`
	BomOutputQty     float64           `json:"bomOutputQty"`
	BomUnit          string            `json:"bomUnit"`
	BomStatus        string            `json:"bomStatus"`
	BomEffectiveDate string            `json:"bomEffectiveDate"`
	Lines            []BOMLineResponse `json:"lines"`
}

type BOMPurchaseRequestInput struct {
	Requester     string `json:"requester"`
	Reason        string `json:"reason"`
	NeededDate    string `json:"neededDate"`
	ProductionQty int    `json:"productionQty"`
}

// GET /api/boms — list standalone BOM records
func ListBOMs(c *fiber.Ctx) error {
	var boms []models.BOM
	if err := database.DB.Order("id DESC").Find(&boms).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(boms)
}

// POST /api/boms — create a standalone BOM record
func CreateBOM(c *fiber.Ctx) error {
	var input models.BOM
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if input.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code is required"})
	}
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	if input.Status == "" {
		input.Status = "Draft"
	}
	if err := database.DB.Create(&input).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(input)
}


// GET /api/boms/:sku?productionQty=5000
func GetBOM(c *fiber.Ctx) error {
	productionQty := c.QueryInt("productionQty", 1)
	if productionQty <= 0 {
		productionQty = 1
	}

	detail, err := buildBOMDetail(c.Params("sku"), productionQty)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "BOM not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(detail)
}

// POST /api/boms/:sku/purchase-request
func CreatePurchaseRequestFromBOM(c *fiber.Ctx) error {
	var input BOMPurchaseRequestInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if input.ProductionQty <= 0 {
		input.ProductionQty = 1
	}
	if input.NeededDate == "" {
		input.NeededDate = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	}
	if input.Requester == "" {
		if username := c.Locals("name"); username != nil {
			input.Requester = username.(string)
		} else {
			input.Requester = "System"
		}
	}

	detail, err := buildBOMDetail(c.Params("sku"), input.ProductionQty)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "BOM not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	items := make([]models.PurchaseRequestItem, 0)
	for _, line := range detail.Lines {
		if !line.CanCreatePR {
			continue
		}
		items = append(items, models.PurchaseRequestItem{
			SKU:  line.SKU,
			Name: line.Name,
			Qty:  int(math.Ceil(line.Shortage)),
			Note: fmt.Sprintf("BOM %s: required %.2f %s, stock %.2f %s", detail.SKU, line.RequiredQty, line.Unit, line.StockQty, line.Unit),
		})
	}
	if len(items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No shortage items to create PR"})
	}

	reason := input.Reason
	if reason == "" {
		reason = fmt.Sprintf("Created from BOM %s (%s), production qty %d", detail.SKU, detail.Name, input.ProductionQty)
	}

	pr := models.PurchaseRequest{
		Requester:  input.Requester,
		Reason:     reason,
		NeededDate: input.NeededDate,
		Items:      items,
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		id, err := NextID(tx, "PR-2026-", &models.PurchaseRequest{}, "id")
		if err != nil {
			return err
		}
		pr.ID = id
		pr.Date = time.Now().Format("2006-01-02")
		pr.Status = "Pending Approval"
		for i := range pr.Items {
			pr.Items[i].RequestID = id
		}
		return tx.Create(&pr).Error
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").First(&pr, "id = ?", pr.ID)
	return c.JSON(pr)
}

func buildBOMDetail(sku string, productionQty int) (BOMDetailResponse, error) {
	var product models.Product
	if err := database.DB.First(&product, "sku = ?", sku).Error; err != nil {
		return BOMDetailResponse{}, err
	}

	var components []models.BundleComponent
	if err := database.DB.Where("bundle_sku = ?", sku).Order("id ASC").Find(&components).Error; err != nil {
		return BOMDetailResponse{}, err
	}

	lines := make([]BOMLineResponse, 0, len(components))
	readyItems := 0
	prRequired := 0
	totalPRValue := 0.0
	totalCostPerUnit := 0.0

	for _, component := range components {
		line, err := buildBOMLine(component, productionQty)
		if err != nil {
			return BOMDetailResponse{}, err
		}
		if line.Shortage == 0 {
			readyItems++
		}
		if line.CanCreatePR {
			prRequired++
			totalPRValue += line.PRValue
		}
		totalCostPerUnit += line.CostPerFinishedUnit
		lines = append(lines, line)
	}

	return BOMDetailResponse{
		SKU:              product.SKU,
		Name:             product.Name,
		ProductionQty:    productionQty,
		ComponentCount:   len(lines),
		PRRequired:       prRequired,
		ReadyItems:       readyItems,
		TotalPRValue:     totalPRValue,
		TotalCostPerUnit: totalCostPerUnit,
		Lines:            lines,
	}, nil
}

func buildBOMLine(component models.BundleComponent, productionQty int) (BOMLineResponse, error) {
	if component.ComponentType == "expense" {
		cost := component.Qty * component.UnitCostOverride
		name := component.ComponentName
		if name == "" {
			name = "ค่าใช้จ่ายใน BOM"
		}
		return BOMLineResponse{
			SKU:                 "",
			Name:                name,
			Category:            "expense",
			Unit:                "บาท",
			QtyPerUnit:          component.Qty,
			RequiredQty:         component.Qty * float64(productionQty),
			StockQty:            0,
			Shortage:            0,
			UnitCost:            component.UnitCostOverride,
			CostPerFinishedUnit: cost,
			PRValue:             0,
			CanCreatePR:         false,
		}, nil
	}

	var componentProduct models.Product
	if err := database.DB.First(&componentProduct, "sku = ?", component.ComponentSku).Error; err != nil {
		return BOMLineResponse{}, err
	}

	componentUnit := component.Unit
	if componentUnit == "" {
		componentUnit = "piece"
	}
	requiredQty := convertBOMQty(component.Qty*float64(productionQty), componentUnit, componentProduct.BaseUnit)
	qtyPerUnit := convertBOMQty(component.Qty, componentUnit, componentProduct.BaseUnit)
	stockQty := float64(componentProduct.Stock)
	shortage := math.Max(requiredQty-stockQty, 0)
	unitCost := componentProduct.Cost
	costPerFinishedUnit := qtyPerUnit * unitCost

	name := component.ComponentName
	if name == "" {
		name = componentProduct.Name
	}

	return BOMLineResponse{
		SKU:                 componentProduct.SKU,
		Name:                name,
		Category:            component.ComponentType,
		Unit:                componentProduct.BaseUnit,
		QtyPerUnit:          qtyPerUnit,
		RequiredQty:         requiredQty,
		StockQty:            stockQty,
		Shortage:            shortage,
		UnitCost:            unitCost,
		CostPerFinishedUnit: costPerFinishedUnit,
		PRValue:             shortage * unitCost,
		CanCreatePR:         shortage > 0,
	}, nil
}

func convertBOMQty(qty float64, fromUnit string, toUnit string) float64 {
	if fromUnit == "g" && toUnit == "kg" {
		return qty / 1000
	}
	if fromUnit == "kg" && toUnit == "g" {
		return qty * 1000
	}
	return qty
}

type SaveBOMInput struct {
	BomCode          string `json:"bomCode"`
	BomName          string `json:"bomName"`
	BomOutputQty     float64 `json:"bomOutputQty"`
	BomUnit          string `json:"bomUnit"`
	BomStatus        string `json:"bomStatus"`
	BomEffectiveDate string `json:"bomEffectiveDate"`
	Components []struct {
		ComponentSku     string  `json:"componentSku"`
		ComponentName    string  `json:"componentName"`
		Qty              float64 `json:"qty"`
		Unit             string  `json:"unit"`
		ComponentType    string  `json:"componentType"`
		UnitCostOverride float64 `json:"unitCostOverride"`
	} `json:"components"`
}

func SaveBOM(c *fiber.Ctx) error {
	sku := c.Params("sku")
	var req SaveBOMInput
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var product models.Product
	if err := database.DB.First(&product, "sku = ?", sku).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		product.IsBundle = true

		// Delete existing components for this bundle
		if err := tx.Where("bundle_sku = ?", sku).Delete(&models.BundleComponent{}).Error; err != nil {
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
				BundleSku:        sku,
				ComponentSku:     comp.ComponentSku,
				ComponentName:    comp.ComponentName,
				Qty:              comp.Qty,
				Unit:             comp.Unit,
				ComponentType:    comp.ComponentType,
				UnitCostOverride: comp.UnitCostOverride,
			}
			if err := tx.Create(&bc).Error; err != nil {
				return err
			}
		}

		// Calculate total cost and update product cost
		var dbComps []models.BundleComponent
		if err := tx.Where("bundle_sku = ?", sku).Find(&dbComps).Error; err != nil {
			return err
		}
		cost, err := calculateBOMCost(tx, dbComps)
		if err != nil {
			return err
		}
		product.Cost = cost
		if err := tx.Save(&product).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	detail, err := buildBOMDetail(sku, int(math.Max(req.BomOutputQty, 1)))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(detail)
}

func RecalculateBOMCost(c *fiber.Ctx) error {
	sku := c.Params("sku")
	var product models.Product
	if err := database.DB.First(&product, "sku = ?", sku).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	var comps []models.BundleComponent
	if err := database.DB.Where("bundle_sku = ?", sku).Find(&comps).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	cost, err := calculateBOMCost(database.DB, comps)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	product.Cost = cost

	if err := database.DB.Save(&product).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	detail, err := buildBOMDetail(sku, 1)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(detail)
}

type DuplicateBOMInput struct {
	TargetSku string `json:"targetSku"`
}

func DuplicateBOM(c *fiber.Ctx) error {
	sku := c.Params("sku")
	var req DuplicateBOMInput
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var sourceProduct models.Product
	if err := database.DB.First(&sourceProduct, "sku = ?", sku).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Source product not found"})
	}

	var targetProduct models.Product
	if err := database.DB.First(&targetProduct, "sku = ?", req.TargetSku).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Target product not found"})
	}

	var comps []models.BundleComponent
	if err := database.DB.Where("bundle_sku = ?", sku).Find(&comps).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		targetProduct.IsBundle = true
		targetProduct.Cost = sourceProduct.Cost

		if err := tx.Save(&targetProduct).Error; err != nil {
			return err
		}

		if err := tx.Where("bundle_sku = ?", req.TargetSku).Delete(&models.BundleComponent{}).Error; err != nil {
			return err
		}

		for _, comp := range comps {
			newComp := models.BundleComponent{
				BundleSku:        req.TargetSku,
				ComponentSku:     comp.ComponentSku,
				ComponentName:    comp.ComponentName,
				Qty:              comp.Qty,
				Unit:             comp.Unit,
				ComponentType:    comp.ComponentType,
				UnitCostOverride: comp.UnitCostOverride,
			}
			if err := tx.Create(&newComp).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	detail, err := buildBOMDetail(req.TargetSku, 1)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(detail)
}


