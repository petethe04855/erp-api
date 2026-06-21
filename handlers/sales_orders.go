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
			return c.JSON(existing)
		}
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Verify and reserve stock
		for _, line := range so.Lines {
			var p models.Product
			if err := tx.First(&p, "sku = ?", line.SKU).Error; err != nil {
				return fmt.Errorf("Product %s not found", line.SKU)
			}
			available := p.Stock - p.ReservedQty
			if available < line.Qty {
				return fmt.Errorf("สต็อคไม่พอ: %s มีพร้อมขาย %d ชิ้น", p.Name, available)
			}

			// Apply reserve qty
			p.ReservedQty += line.Qty
			if err := tx.Save(&p).Error; err != nil {
				return err
			}
		}

		// Generate ID
		id, err := NextID(tx, "SO-2026-", &models.SalesOrder{}, "id")
		if err != nil {
			return err
		}
		so.ID = id
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

		for i := range so.Lines {
			so.Lines[i].OrderID = id
		}

		if err := tx.Create(&so).Error; err != nil {
			return err
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

	database.DB.Preload("Lines").Preload("AuditTrail").First(&so, "id = ?", so.ID)
	return c.JSON(so)
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
	if err := database.DB.Preload("Lines").First(&so, "id = ?", id).Error; err != nil {
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
		so.Status = newStatus

		if err := tx.Save(&so).Error; err != nil {
			return err
		}

		// Release reserved qty if Cancelled or Completed
		if (newStatus == "Cancelled" || newStatus == "Completed") && len(so.Lines) > 0 {
			for _, line := range so.Lines {
				var p models.Product
				if err := tx.First(&p, "sku = ?", line.SKU).Error; err == nil {
					p.ReservedQty = p.ReservedQty - line.Qty
					if p.ReservedQty < 0 {
						p.ReservedQty = 0
					}
					if err := tx.Save(&p).Error; err != nil {
						return err
					}
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

				if product.IsBundle {
					// Expand bundle into components
					var comps []models.BundleComponent
					if err := tx.Where("bundle_sku = ?", product.SKU).Find(&comps).Error; err == nil {
						for _, comp := range comps {
							if comp.ComponentType == "expense" {
								continue
							}
							var componentProduct models.Product
							if err := tx.First(&componentProduct, "sku = ?", comp.ComponentSku).Error; err != nil {
								return err
							}
							required := comp.Qty
							if comp.Unit == "g" && componentProduct.BaseUnit == "kg" {
								required /= 1000
							} else if comp.Unit == "kg" && componentProduct.BaseUnit == "g" {
								required *= 1000
							}
							neededQty := int(math.Ceil(required * float64(line.Qty)))
							if err := deductFefoStock(tx, comp.ComponentSku, neededQty, so.ID, username.(string)); err != nil {
								return err
							}
						}
					}
				} else {
					if err := deductFefoStock(tx, line.SKU, line.Qty, so.ID, username.(string)); err != nil {
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

	database.DB.Preload("Lines").Preload("AuditTrail").First(&so, "id = ?", so.ID)
	return c.JSON(so)
}

// Helper: Deduct stock from earliest expiry lots (FEFO)
func deductFefoStock(tx *gorm.DB, sku string, qty int, refDoc string, by string) error {
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
			ID:        fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), lot.Lot),
			SKU:       sku,
			Type:      "OUT",
			Qty:       deduct,
			RefDoc:    refDoc,
			Date:      time.Now().Format("2006-01-02"),
			Note:      fmt.Sprintf("FEFO: lot %s exp %s", lot.Lot, lot.ExpiryDate),
			ChangedBy: by,
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}
	}

	// Update overall product stock count
	var prod models.Product
	if err := tx.First(&prod, "sku = ?", sku).Error; err == nil {
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

		id, err := NextID(tx, "INV-2026-", &models.Invoice{}, "id")
		if err != nil {
			return err
		}
		inv.ID = id
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

	database.DB.Preload("AuditTrail").First(&inv, "id = ?", inv.ID)
	return c.JSON(inv)
}

// POST /api/invoices/from-so/:soId
func CreateInvoiceFromSO(c *fiber.Ctx) error {
	soID := c.Params("soId")
	var so models.SalesOrder
	if err := database.DB.Preload("Lines").First(&so, "id = ?", soID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Sales order not found"})
	}

	var inv models.Invoice
	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&models.Invoice{}).Where("so_ref = ?", soID).Count(&count)
		if count > 0 {
			tx.Preload("AuditTrail").First(&inv, "so_ref = ?", soID)
			return nil
		}

		id, err := NextID(tx, "INV-2026-", &models.Invoice{}, "id")
		if err != nil {
			return err
		}

		inv = models.Invoice{
			ID:        id,
			SoRef:     so.ID,
			Customer:  so.Customer,
			IssueDate: time.Now().Format("2006-01-02"),
			DueDate:   time.Now().AddDate(0, 0, 14).Format("2006-01-02"),
			Amount:    so.Amount,
			Paid:      0,
			Status:    "Unpaid",
		}

		if err := tx.Create(&inv).Error; err != nil {
			return err
		}

		// Update sales order invRef link
		so.InvRef = inv.ID
		if err := tx.Save(&so).Error; err != nil {
			return err
		}

		audit := models.AuditEvent{
			OwnerID:   inv.ID,
			OwnerType: "invoices",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("สร้างใบแจ้งหนี้จาก SO %s", soID),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("AuditTrail").First(&inv, "id = ?", inv.ID)
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
	if err := database.DB.Preload("AuditTrail").First(&inv, "id = ?", id).Error; err != nil {
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

	database.DB.Preload("AuditTrail").First(&inv, "id = ?", inv.ID)
	return c.JSON(inv)
}
