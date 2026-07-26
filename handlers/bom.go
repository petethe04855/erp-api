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

type BOMComponentInput struct {
	ComponentSku      string  `json:"componentSku"`
	ComponentName     string  `json:"componentName"`
	Qty               float64 `json:"qty"`
	Unit              string  `json:"unit"`
	ScrapRate         float64 `json:"scrapRate"`
	BOMLevel          int     `json:"bomLevel"`
	Description       string  `json:"description"`
	ProcurementMethod string  `json:"procurementMethod"`
	Note              string  `json:"note"`
	ComponentType     string  `json:"componentType"`
	UnitCostOverride  float64 `json:"unitCostOverride"`
	YieldFactor       float64 `json:"yieldFactor"`
}

type BOMCreateInput struct {
	models.BOM
	Components []BOMComponentInput `json:"components"`
}

type BOMComponentResponse struct {
	ComponentSku      string  `json:"componentSku"`
	ComponentName     string  `json:"componentName"`
	Qty               float64 `json:"qty"`
	Unit              string  `json:"unit"`
	ScrapRate         float64 `json:"scrapRate"`
	BOMLevel          int     `json:"bomLevel"`
	Description       string  `json:"description"`
	ProcurementMethod string  `json:"procurementMethod"`
	Note              string  `json:"note"`
	ComponentType     string  `json:"componentType"`
	UnitCostOverride  float64 `json:"unitCostOverride"`
	YieldFactor       float64 `json:"yieldFactor"`
	UnitCost          float64 `json:"unitCost"`
	LineCost          float64 `json:"lineCost"`
	IsSubComponent    bool    `json:"isSubComponent"`
}

type BOMListResponse struct {
	models.BOM
	Components []BOMComponentResponse `json:"components"`
}

// GET /api/boms — list standalone BOM records
func ListBOMs(c *fiber.Ctx) error {
	var boms []models.BOM
	if err := database.DB.Order("id DESC").Find(&boms).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	result := make([]BOMListResponse, 0, len(boms))
	for _, bom := range boms {
		components, _ := buildStandaloneBOMComponents(database.DB, bom)
		result = append(result, BOMListResponse{BOM: bom, Components: components})
	}
	return c.JSON(result)
}

// POST /api/boms — create a standalone BOM record
func CreateBOM(c *fiber.Ctx) error {
	var input BOMCreateInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if input.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "code is required"})
	}
	if input.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	if input.FgSku == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "fgSku is required"})
	}
	if input.OutputQty <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "outputQty must be greater than zero"})
	}
	if len(input.Components) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "BOM requires at least one component"})
	}
	if input.Status == "" {
		input.Status = "Active"
	}
	if input.Kind == "" {
		input.Kind = "finished"
	}
	if input.Version <= 0 {
		input.Version = 1
	}

	var output models.Product
	if err := database.DB.First(&output, "sku = ?", input.FgSku).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "fgSku must exist in Item Master"})
	}
	if input.Kind == "subcomponent" && output.Type != "Sub-component" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subcomponent BOM output must use a Sub-component item"})
	}
	if input.Kind != "subcomponent" && output.Type != "Finished Product" && output.Type != "Bundle" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "finished BOM output must use a Finished Product item"})
	}

	bom := input.BOM
	bom.Cost = 0
	bom.ComponentCount = len(input.Components)

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if bom.Status == "Active" {
			if err := tx.Model(&models.BOM{}).
				Where("fg_sku = ? AND status = 'Active'", bom.FgSku).
				Update("status", "Inactive").Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&bom).Error; err != nil {
			return err
		}
		if err := tx.Where("bundle_sku = ?", bom.FgSku).Delete(&models.BundleComponent{}).Error; err != nil {
			return err
		}
		components := make([]models.BundleComponent, 0, len(input.Components))
		for _, comp := range input.Components {
			bc, err := validateAndBuildBOMComponent(tx, bom.FgSku, comp)
			if err != nil {
				return err
			}
			if err := tx.Create(&bc).Error; err != nil {
				return err
			}
			components = append(components, bc)
		}
		cost, err := calculateStandaloneBOMCost(tx, bom, components)
		if err != nil {
			return err
		}
		bom.Cost = cost
		if err := tx.Save(&bom).Error; err != nil {
			return err
		}
		output.IsBundle = true
		output.Cost = cost / bom.OutputQty
		return tx.Save(&output).Error
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	components, _ := buildStandaloneBOMComponents(database.DB, bom)
	return c.Status(fiber.StatusCreated).JSON(BOMListResponse{BOM: bom, Components: components})
}

// DELETE /api/boms/:id — delete a BOM record by ID or Code
func DeleteBOM(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID or Code is required"})
	}
	if err := database.DB.Where("id = ? OR code = ?", id, id).Delete(&models.BOM{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "BOM deleted successfully"})
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
		code, err := NextCode(tx, "PR-2026-", &models.PurchaseRequest{}, "code")
		if err != nil {
			return err
		}
		pr.Code = code
		pr.Date = time.Now().Format("2006-01-02")
		pr.Status = "Pending Approval"
		if err := tx.Create(&pr).Error; err != nil {
			return err
		}
		for i := range pr.Items {
			pr.Items[i].PurchaseRequestID = pr.ID
			tx.Model(&pr.Items[i]).Update("purchase_request_id", pr.ID)
		}
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").First(&pr, pr.ID)
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
	batchSize := componentQtyBatchSize(component.BundleSku)
	netRequiredQty := component.Qty * (float64(productionQty) / batchSize)
	requiredQty := convertBOMQty(grossBOMQty(netRequiredQty, component.ScrapRate), componentUnit, componentProduct.BaseUnit)
	qtyPerUnit := convertBOMQty(grossBOMQty(component.Qty/batchSize, component.ScrapRate), componentUnit, componentProduct.BaseUnit)
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

func grossBOMQty(netQty float64, scrapRate float64) float64 {
	if scrapRate <= 0 {
		return netQty
	}
	return netQty / (1 - (scrapRate / 100))
}

func componentQtyBatchSize(fgSku string) float64 {
	var bom models.BOM
	if err := database.DB.Where("fg_sku = ? AND status = 'Active'", fgSku).First(&bom).Error; err == nil && bom.OutputQty > 0 {
		return bom.OutputQty
	}
	return 1
}

type SaveBOMInput struct {
	BomCode          string  `json:"bomCode"`
	BomName          string  `json:"bomName"`
	BomOutputQty     float64 `json:"bomOutputQty"`
	BomUnit          string  `json:"bomUnit"`
	BomStatus        string  `json:"bomStatus"`
	BomEffectiveDate string  `json:"bomEffectiveDate"`
	Components       []struct {
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

func validateAndBuildBOMComponent(tx *gorm.DB, bundleSku string, comp BOMComponentInput) (models.BundleComponent, error) {
	if comp.Qty <= 0 {
		return models.BundleComponent{}, fmt.Errorf("component qty must be greater than zero")
	}
	if comp.Unit == "" {
		comp.Unit = "piece"
	}
	if comp.ComponentType == "" {
		comp.ComponentType = "material"
	}
	if comp.ScrapRate < 0 || comp.ScrapRate >= 100 {
		return models.BundleComponent{}, fmt.Errorf("component scrapRate must be >= 0 and < 100")
	}
	if comp.BOMLevel <= 0 {
		comp.BOMLevel = 1
	}
	if comp.ProcurementMethod == "" {
		comp.ProcurementMethod = "Buy"
	}
	if comp.YieldFactor <= 0 {
		comp.YieldFactor = 1
	}
	if comp.ComponentType != "expense" {
		if comp.ComponentSku == "" {
			return models.BundleComponent{}, fmt.Errorf("component SKU is required")
		}
		var componentProduct models.Product
		if err := tx.First(&componentProduct, "sku = ?", comp.ComponentSku).Error; err != nil {
			return models.BundleComponent{}, fmt.Errorf("component SKU %s must exist in Item Master", comp.ComponentSku)
		}
		if componentProduct.Type != "Raw Material" &&
			componentProduct.Type != "Packaging" &&
			componentProduct.Type != "Sub-component" &&
			componentProduct.Type != "Bundle" &&
			componentProduct.Type != "Cat" &&
			componentProduct.Type != "Dog" &&
			componentProduct.Type != "Other" {
			return models.BundleComponent{}, fmt.Errorf("component SKU %s has unsupported item type %s", comp.ComponentSku, componentProduct.Type)
		}
		if comp.ComponentName == "" {
			comp.ComponentName = componentProduct.Name
		}
	}

	return models.BundleComponent{
		BundleSku:         bundleSku,
		ComponentSku:      comp.ComponentSku,
		ComponentName:     comp.ComponentName,
		Qty:               comp.Qty,
		Unit:              comp.Unit,
		ScrapRate:         comp.ScrapRate,
		BOMLevel:          comp.BOMLevel,
		Description:       comp.Description,
		ProcurementMethod: comp.ProcurementMethod,
		Note:              comp.Note,
		ComponentType:     comp.ComponentType,
		UnitCostOverride:  comp.UnitCostOverride,
		YieldFactor:       comp.YieldFactor,
	}, nil
}

func calculateStandaloneBOMCost(tx *gorm.DB, bom models.BOM, components []models.BundleComponent) (float64, error) {
	total := 0.0
	for _, comp := range components {
		lineCost, err := calculateStandaloneBOMLineCost(tx, bom, comp)
		if err != nil {
			return 0, err
		}
		total += lineCost
	}
	return total, nil
}

func calculateStandaloneBOMLineCost(tx *gorm.DB, bom models.BOM, comp models.BundleComponent) (float64, error) {
	if comp.ComponentType == "expense" {
		return comp.Qty * comp.UnitCostOverride, nil
	}
	var product models.Product
	if err := tx.First(&product, "sku = ?", comp.ComponentSku).Error; err != nil {
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
	return qty * unitCost, nil
}

func buildStandaloneBOMComponents(tx *gorm.DB, bom models.BOM) ([]BOMComponentResponse, error) {
	if bom.FgSku == "" {
		return []BOMComponentResponse{}, nil
	}
	var components []models.BundleComponent
	if err := tx.Where("bundle_sku = ?", bom.FgSku).Order("id ASC").Find(&components).Error; err != nil {
		return nil, err
	}
	result := make([]BOMComponentResponse, 0, len(components))
	for _, comp := range components {
		unitCost := comp.UnitCostOverride
		name := comp.ComponentName
		isSubComponent := false
		if comp.ComponentType != "expense" && comp.ComponentSku != "" {
			var product models.Product
			if err := tx.First(&product, "sku = ?", comp.ComponentSku).Error; err == nil {
				if name == "" {
					name = product.Name
				}
				unitCost = product.Cost
				isSubComponent = product.Type == "Sub-component" || product.IsBundle
			}
		}
		lineCost, _ := calculateStandaloneBOMLineCost(tx, bom, comp)
		result = append(result, BOMComponentResponse{
			ComponentSku:      comp.ComponentSku,
			ComponentName:     name,
			Qty:               comp.Qty,
			Unit:              comp.Unit,
			ScrapRate:         comp.ScrapRate,
			BOMLevel:          comp.BOMLevel,
			Description:       comp.Description,
			ProcurementMethod: comp.ProcurementMethod,
			Note:              comp.Note,
			ComponentType:     comp.ComponentType,
			UnitCostOverride:  comp.UnitCostOverride,
			YieldFactor:       comp.YieldFactor,
			UnitCost:          unitCost,
			LineCost:          lineCost,
			IsSubComponent:    isSubComponent,
		})
	}
	return result, nil
}
