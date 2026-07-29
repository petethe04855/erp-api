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

// POST /api/sales-orders
func CreateSalesOrder(c *fiber.Ctx) error {
	var so models.SalesOrder
	if err := c.BodyParser(&so); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if so.Customer == "" || so.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "SO requires customer and amount > 0"})
	}

	// Check if qtRef already converted
	if so.QtRef != "" {
		var existing models.SalesOrder
		if err := database.DB.Where("qt_ref = ?", so.QtRef).First(&existing).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "quotation has already been converted to Sales Order " + existing.Code})
		}
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Verify and reserve stock
		for i, line := range so.Lines {
			var p models.Product
			if err := tx.First(&p, "sku = ?", line.SKU).Error; err != nil {
				return fmt.Errorf("Product %s not found", line.SKU)
			}
			if !isSellableProduct(p) {
				return fmt.Errorf("sales item %s must be a Finished Product", line.SKU)
			}
			so.Lines[i].ProductID = p.ID
			if err := reserveSalesStock(tx, p, line.Qty, 1); err != nil {
				return err
			}
		}

		code, err := NextCode(tx, "SO-2026-", &models.SalesOrder{}, "code")
		if err != nil {
			return err
		}
		so.Code = code
		so.Items = len(so.Lines)

		if so.Date == "" {
			so.Date = time.Now().Format("2006-01-02")
		}
		if so.Status == "" {
			so.Status = "Pending"
		}
		if so.Channel == "" {
			so.Channel = "Manual"
		}

		if err := tx.Create(&so).Error; err != nil {
			return err
		}

		for i := range so.Lines {
			so.Lines[i].SalesOrderID = so.ID
			tx.Model(&so.Lines[i]).Update("sales_order_id", so.ID)
		}

		// Create Audit Event
		audit := models.AuditEvent{
			OwnerID:   so.ID,
			OwnerType: "sales_orders",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("สร้าง SO ช่องทาง %s", so.Channel),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Lines").Preload("AuditTrail").First(&so, so.ID)
	return c.JSON(so)
}

// reserveSalesStock reserves physical inventory for a sales line. Bundle sales
// reserve their BOM components, so available stock stays consistent with stock
// checking, transfers, and the later FEFO deduction.
func reserveSalesStock(tx *gorm.DB, product models.Product, qty int, direction int) error {
	type reservation struct {
		product models.Product
		qty     int
	}
	reservations := []reservation{}

	requirements, err := expandBOMMaterialRequirements(tx, product.SKU, float64(qty), map[string]bool{})
	if err != nil {
		return err
	}
	for sku, requiredQty := range requirements {
		var component models.Product
		if err := tx.First(&component, "sku = ?", sku).Error; err != nil {
			return err
		}
		reservations = append(reservations, reservation{product: component, qty: int(math.Ceil(requiredQty))})
	}

	for _, item := range reservations {
		if direction > 0 && item.product.Stock-item.product.ReservedQty < item.qty {
			available := item.product.Stock - item.product.ReservedQty
			return fmt.Errorf("สต็อคไม่พอ: %s ต้องใช้ %d %s, พร้อมใช้ %d %s", item.product.Name, item.qty, item.product.BaseUnit, available, item.product.BaseUnit)
		}
		item.product.ReservedQty += direction * item.qty
		if item.product.ReservedQty < 0 {
			item.product.ReservedQty = 0
		}
		if err := tx.Save(&item.product).Error; err != nil {
			return err
		}
	}
	return nil
}

// PUT /api/sales-orders/:id/status
func UpdateSalesOrderStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var so models.SalesOrder
	if err := ByIDOrCode(database.DB, id).Preload("Lines").First(&so).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sales order not found"})
	}

	if so.Status == "Cancelled" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot update cancelled order"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		oldStatus := so.Status
		newStatus := req.Status
		if oldStatus == newStatus {
			return nil
		}
		so.Status = newStatus

		if err := tx.Save(&so).Error; err != nil {
			return err
		}

		// Release reserved qty if Cancelled or Completed
		if (newStatus == "Cancelled" || newStatus == "Completed") && len(so.Lines) > 0 {
			for _, line := range so.Lines {
				var p models.Product
				if err := tx.First(&p, "sku = ?", line.SKU).Error; err != nil {
					return err
				}
				if err := reserveSalesStock(tx, p, line.Qty, -1); err != nil {
					return err
				}
			}
		}

		// FEFO Stock deduction when completed
		if newStatus == "Completed" && len(so.Lines) > 0 {
			for _, line := range so.Lines {
				var product models.Product
				if err := tx.First(&product, "sku = ?", line.SKU).Error; err != nil {
					return err
				}

				requirements, err := expandBOMMaterialRequirements(tx, product.SKU, float64(line.Qty), map[string]bool{})
				if err != nil {
					return err
				}
				for sku, requiredQty := range requirements {
					if err := deductFefoStock(tx, sku, int(math.Ceil(requiredQty)), so.Code, &so.ID, "sales_orders", username.(string)); err != nil {
						return err
					}
				}
			}
		}

		// Record Audit Event
		audit := models.AuditEvent{
			OwnerID:   so.ID,
			OwnerType: "sales_orders",
			Action:    newStatus,
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("เปลี่ยนสถานะจาก %s เป็น %s", oldStatus, newStatus),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Lines").Preload("AuditTrail").First(&so, so.ID)
	return c.JSON(so)
}

// Helper: Deduct stock from earliest expiry lots (FEFO)
func deductFefoStock(tx *gorm.DB, sku string, qty int, refDocCode string, refDocID *uint, refDocType string, by string) error {
	var prod models.Product
	if err := tx.First(&prod, "sku = ?", sku).Error; err != nil {
		return err
	}
	if err := ensureLotBalance(tx, prod); err != nil {
		return err
	}

	var lots []models.StockLot
	// Query non-empty lots, sorting by expiryDate ascending, empty dates at the end
	err := tx.Where("sku = ? AND remaining_qty > 0", sku).
		Order("CASE WHEN expiry_date IS NULL OR expiry_date = '' THEN 1 ELSE 0 END, expiry_date ASC").
		Find(&lots).Error
	if err != nil {
		return err
	}

	rem := qty
	for i := range lots {
		if rem <= 0 {
			break
		}
		lot := &lots[i]
		deduct := lot.RemainingQty
		if rem < deduct {
			deduct = rem
		}
		lot.RemainingQty -= deduct
		rem -= deduct

		if err := tx.Save(lot).Error; err != nil {
			return err
		}

		// Create Stock Movement
		movement := models.StockMovement{
			Code:       fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), lot.Lot),
			ProductID:  prod.ID,
			SKU:        sku,
			Type:       "OUT",
			Qty:        deduct,
			RefDoc:     refDocCode,
			RefDocType: refDocType,
			RefDocID:   refDocID,
			Date:       time.Now().Format("2006-01-02"),
			Note:       fmt.Sprintf("FEFO: lot %s exp %s", lot.Lot, lot.ExpiryDate),
			ChangedBy:  by,
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}
	}

	// Update overall product stock count
	if rem > 0 {
		return fmt.Errorf("lot stock not sufficient for %s: missing %d", sku, rem)
	}

	if prod.ID > 0 {
		prod.Stock -= qty
		if prod.Stock < 0 {
			prod.Stock = 0
		}
		if err := tx.Save(&prod).Error; err != nil {
			return err
		}
	}

	return nil
}

// ensureLotBalance creates a traceable opening-balance lot for legacy stock that
// exists in Product.Stock but was never recorded at lot level. It never changes
// the product quantity and refuses to conceal the opposite inconsistency.
func ensureLotBalance(tx *gorm.DB, product models.Product) error {
	var lotQty int
	if err := tx.Model(&models.StockLot{}).
		Where("sku = ?", product.SKU).
		Select("COALESCE(SUM(remaining_qty), 0)").
		Scan(&lotQty).Error; err != nil {
		return err
	}
	if lotQty > product.Stock {
		return fmt.Errorf("lot balance exceeds product stock for %s: lots %d, product %d", product.SKU, lotQty, product.Stock)
	}
	if lotQty == product.Stock {
		return nil
	}

	difference := product.Stock - lotQty
	openingLot := models.StockLot{
		Code:           fmt.Sprintf("LOT-OPENING-%s", product.SKU),
		ProductID:      product.ID,
		SKU:            product.SKU,
		Lot:            fmt.Sprintf("OPENING-%s", product.SKU),
		Qty:            difference,
		RemainingQty:   difference,
		LandedUnitCost: product.Cost,
		ReceivedDate:   time.Now().Format("2006-01-02"),
		GrRef:          "OPENING_BALANCE",
	}
	return tx.Create(&openingLot).Error
}

func isSellableProduct(product models.Product) bool {
	return product.Type == "Finished Product" || product.Type == "Bundle" || product.Type == "Cat" || product.Type == "Dog"
}

// POST /api/invoices
func CreateInvoice(c *fiber.Ctx) error {
	var inv models.Invoice
	if err := c.BodyParser(&inv); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if inv.Customer == "" || inv.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invoice requires customer and amount > 0"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if inv.SoRef != "" {
			var count int64
			tx.Model(&models.Invoice{}).Where("so_ref = ?", inv.SoRef).Count(&count)
			if count > 0 {
				var existing models.Invoice
				tx.Preload("AuditTrail").First(&existing, "so_ref = ?", inv.SoRef)
				inv = existing
				return nil
			}
		}

		code, err := NextCode(tx, "INV-2026-", &models.Invoice{}, "code")
		if err != nil {
			return err
		}
		inv.Code = code
		if inv.IssueDate == "" {
			inv.IssueDate = time.Now().Format("2006-01-02")
		}
		if inv.DueDate == "" {
			inv.DueDate = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
		}
		inv.Paid = 0
		if inv.Status == "" {
			inv.Status = "Unpaid"
		}

		if err := tx.Create(&inv).Error; err != nil {
			return err
		}

		audit := models.AuditEvent{
			OwnerID:   inv.ID,
			OwnerType: "invoices",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      "สร้างใบแจ้งหนี้",
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("AuditTrail").First(&inv, inv.ID)
	return c.JSON(inv)
}

// POST /api/invoices/from-so/:soId
func CreateInvoiceFromSO(c *fiber.Ctx) error {
	soID := c.Params("soId")
	var so models.SalesOrder
	if err := ByIDOrCode(database.DB, soID).Preload("Lines").First(&so).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sales order not found"})
	}

	var inv models.Invoice
	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&models.Invoice{}).Where("sales_order_id = ? OR so_ref = ?", so.ID, so.Code).Count(&count)
		if count > 0 {
			tx.Preload("AuditTrail").First(&inv, "sales_order_id = ? OR so_ref = ?", so.ID, so.Code)
			return nil
		}

		code, err := NextCode(tx, "INV-2026-", &models.Invoice{}, "code")
		if err != nil {
			return err
		}

		inv = models.Invoice{
			Code:         code,
			SalesOrderID: &so.ID,
			SoRef:        so.Code,
			Customer:     so.Customer,
			IssueDate:    time.Now().Format("2006-01-02"),
			DueDate:      time.Now().AddDate(0, 0, 14).Format("2006-01-02"),
			Amount:       so.Amount,
			Paid:         0,
			Status:       "Unpaid",
		}

		if err := tx.Create(&inv).Error; err != nil {
			return err
		}

		// Update sales order invRef link
		so.InvRef = inv.Code
		if err := tx.Save(&so).Error; err != nil {
			return err
		}

		audit := models.AuditEvent{
			OwnerID:   inv.ID,
			OwnerType: "invoices",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("สร้างใบแจ้งหนี้จาก SO %s", so.Code),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("AuditTrail").First(&inv, inv.ID)
	return c.JSON(inv)
}

// POST /api/invoices/:id/payment
func RecordPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var inv models.Invoice
	if err := ByIDOrCode(database.DB, id).Preload("AuditTrail").First(&inv).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Invoice not found"})
	}

	if inv.Status == "Paid" || req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payment registration"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		inv.Paid = math.Min(inv.Paid+req.Amount, inv.Amount)
		if inv.Paid >= inv.Amount {
			inv.Status = "Paid"
		} else {
			inv.Status = "Partial"
		}

		if err := tx.Save(&inv).Error; err != nil {
			return err
		}

		audit := models.AuditEvent{
			OwnerID:   inv.ID,
			OwnerType: "invoices",
			Action:    "Payment",
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("รับชำระ ฿%s", fmt.Sprintf("%.2f", req.Amount)),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("AuditTrail").First(&inv, inv.ID)
	return c.JSON(inv)
}
