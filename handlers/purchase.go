package handlers

import (
	"fmt"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// POST /api/purchase-requests
func CreatePurchaseRequest(c *fiber.Ctx) error {
	var pr models.PurchaseRequest
	if err := c.BodyParser(&pr); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if pr.Requester == "" || pr.NeededDate == "" || len(pr.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PR requires requester, neededDate, and at least one item"})
	}

	for _, item := range pr.Items {
		if item.Qty <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "All PR items must have qty > 0"})
		}
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
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

		if err := tx.Create(&pr).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").First(&pr, "id = ?", pr.ID)
	return c.JSON(pr)
}

// PUT /api/purchase-requests/:id/status
func UpdatePRStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var pr models.PurchaseRequest
	if err := database.DB.First(&pr, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "PR not found"})
	}

	pr.Status = req.Status
	if err := database.DB.Save(&pr).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").First(&pr, "id = ?", id)
	return c.JSON(pr)
}

type ConvertPRtoPORequest struct {
	Supplier  string             `json:"supplier"`
	EtaDate   string             `json:"etaDate"`
	ItemCosts map[string]float64 `json:"itemCosts"`
}

// POST /api/purchase-requests/:id/convert
func ConvertPRtoPO(c *fiber.Ctx) error {
	id := c.Params("id")
	var pr models.PurchaseRequest
	if err := database.DB.Preload("Items").First(&pr, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "PR not found"})
	}

	if pr.Status != "Approved" || pr.PoRef != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PR must be Approved and not converted already"})
	}

	var req ConvertPRtoPORequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	var po models.PurchaseOrder
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		poID, err := NextID(tx, "PO-2026-", &models.PurchaseOrder{}, "id")
		if err != nil {
			return err
		}

		var poItems []models.PurchaseOrderItem
		var totalCost float64
		for _, item := range pr.Items {
			cost := req.ItemCosts[item.SKU]
			poItems = append(poItems, models.PurchaseOrderItem{
				OrderID:     poID,
				SKU:         item.SKU,
				Name:        item.Name,
				Qty:         item.Qty,
				UnitCost:    cost,
				ReceivedQty: 0,
			})
			totalCost += float64(item.Qty) * cost
		}

		po = models.PurchaseOrder{
			ID:        poID,
			Supplier:  req.Supplier,
			EtaDate:   req.EtaDate,
			Date:      time.Now().Format("2006-01-02"),
			Items:     poItems,
			Status:    "Draft",
			PrRef:     pr.ID,
			TotalCost: totalCost,
		}

		if err := tx.Create(&po).Error; err != nil {
			return err
		}

		// Update PR poRef
		pr.PoRef = poID
		if err := tx.Save(&pr).Error; err != nil {
			return err
		}

		// Create Audit Event
		audit := models.AuditEvent{
			OwnerID:   po.ID,
			OwnerType: "purchase_orders",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("แปลงจาก %s", pr.ID),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").Preload("AuditTrail").First(&po, "id = ?", po.ID)
	return c.JSON(po)
}

// POST /api/purchase-orders
func CreatePurchaseOrder(c *fiber.Ctx) error {
	var po models.PurchaseOrder
	if err := c.BodyParser(&po); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if po.Supplier == "" || po.EtaDate == "" || len(po.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PO requires supplier, etaDate, and at least one item"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		id, err := NextID(tx, "PO-2026-", &models.PurchaseOrder{}, "id")
		if err != nil {
			return err
		}
		po.ID = id
		po.Date = time.Now().Format("2006-01-02")
		po.Status = "Draft"

		var totalCost float64
		for i := range po.Items {
			po.Items[i].OrderID = id
			po.Items[i].ReceivedQty = 0
			totalCost += float64(po.Items[i].Qty) * po.Items[i].UnitCost
		}
		po.TotalCost = totalCost

		if err := tx.Create(&po).Error; err != nil {
			return err
		}

		audit := models.AuditEvent{
			OwnerID:   po.ID,
			OwnerType: "purchase_orders",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      "สร้าง Purchase Order",
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").Preload("AuditTrail").First(&po, "id = ?", po.ID)
	return c.JSON(po)
}

// PUT /api/purchase-orders/:id/status
func UpdatePOStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var po models.PurchaseOrder
	if err := database.DB.Preload("Items").First(&po, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "PO not found"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		oldStatus := po.Status
		po.Status = req.Status
		if err := tx.Save(&po).Error; err != nil {
			return err
		}

		audit := models.AuditEvent{
			OwnerID:   po.ID,
			OwnerType: "purchase_orders",
			Action:    req.Status,
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("เปลี่ยนสถานะจาก %s เป็น %s", oldStatus, req.Status),
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").Preload("AuditTrail").First(&po, "id = ?", id)
	return c.JSON(po)
}

// POST /api/goods-receives
func CreateGoodsReceive(c *fiber.Ctx) error {
	var gr models.GoodsReceive
	if err := c.BodyParser(&gr); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if gr.PoRef == "" || gr.ReceiveDate == "" || len(gr.Items) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "GR requires poRef, receiveDate, and at least one item"})
	}

	var po models.PurchaseOrder
	if err := database.DB.Preload("Items").First(&po, "id = ?", gr.PoRef).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Matching PO not found"})
	}

	if po.Status != "Sent" && po.Status != "Partial Received" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PO must be Sent or Partial Received status to receive goods"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Generate GR ID
		id, err := NextID(tx, "GR-2026-", &models.GoodsReceive{}, "id")
		if err != nil {
			return err
		}
		gr.ID = id

		// Assign ReceiveID to LandedCosts
		for idx := range gr.LandedCosts {
			gr.LandedCosts[idx].ReceiveID = id
		}

		// รวมค่า landed ที่ allocatable
		totalLanded := 0.0
		for _, lc := range gr.LandedCosts {
			if lc.Allocatable {
				totalLanded += lc.Amount
			}
		}

		// มูลค่ารวมของที่รับ (ใช้ราคา PO เป็นฐานการปัน)
		totalValue := 0.0
		for i := range gr.Items {
			var poItem *models.PurchaseOrderItem
			for j := range po.Items {
				if po.Items[j].SKU == gr.Items[i].SKU {
					poItem = &po.Items[j]
					break
				}
			}
			if poItem != nil {
				totalValue += float64(gr.Items[i].QtyReceived) * poItem.UnitCost
			}
		}

		for i := range gr.Items {
			item := &gr.Items[i]
			item.ReceiveID = id

			if item.QtyReceived <= 0 {
				return fmt.Errorf("Invalid QtyReceived for item %s", item.SKU)
			}

			// Verify with PO item limits
			var poItem *models.PurchaseOrderItem
			for j := range po.Items {
				if po.Items[j].SKU == item.SKU {
					poItem = &po.Items[j]
					break
				}
			}

			if poItem == nil {
				return fmt.Errorf("Item %s not found in PO %s", item.SKU, gr.PoRef)
			}

			if item.QtyReceived > (poItem.Qty - poItem.ReceivedQty) {
				return fmt.Errorf("Cannot receive more than ordered qty for item %s", item.SKU)
			}

			// มูลค่าของบรรทัดนี้
			lineValue := float64(item.QtyReceived) * poItem.UnitCost

			// ค่าขนส่งที่ปันมาที่บรรทัดนี้
			allocatedFreight := 0.0
			if totalValue > 0 {
				allocatedFreight = totalLanded * (lineValue / totalValue)
			}

			// ต้นทุนรวมต่อหน่วย = (ราคาซื้อ + ค่าขนส่งปัน) / จำนวน
			item.LandedUnitCost = (lineValue + allocatedFreight) / float64(item.QtyReceived)

			// Update PO item received quantity
			poItem.ReceivedQty += item.QtyReceived
			if err := tx.Save(poItem).Error; err != nil {
				return err
			}

			// Create Stock Lot
			lot := models.StockLot{
				ID:           fmt.Sprintf("LOT-%d-%s", time.Now().UnixNano(), item.Lot),
				SKU:          item.SKU,
				Lot:          item.Lot,
				Qty:          item.QtyReceived,
				RemainingQty: item.QtyReceived,
				ExpiryDate:   item.ExpiryDate,
				ReceivedDate: gr.ReceiveDate,
				GrRef:        id,
				PoRef:        gr.PoRef,
			}
			if err := tx.Create(&lot).Error; err != nil {
				return err
			}

			// Update product stock balance
			var p models.Product
			if err := tx.First(&p, "sku = ?", item.SKU).Error; err == nil {
				oldStock := p.Stock
				if oldStock+item.QtyReceived > 0 {
					p.Cost = ((float64(oldStock) * p.Cost) + (float64(item.QtyReceived) * item.LandedUnitCost)) /
						float64(oldStock+item.QtyReceived)
				}
				p.Stock += item.QtyReceived
				if err := tx.Save(&p).Error; err != nil {
					return err
				}

				// Recalculate any parent BOMs that use this raw material SKU
				var parentComponents []models.BundleComponent
				if err := tx.Where("component_sku = ?", item.SKU).Find(&parentComponents).Error; err == nil {
					for _, pc := range parentComponents {
						_ = recalculateProductBOMCost(tx, pc.BundleSku)
					}
				}
			}

			// Create Stock Movement
			movement := models.StockMovement{
				ID:        fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), item.SKU),
				SKU:       item.SKU,
				Type:      "IN",
				Qty:       item.QtyReceived,
				RefDoc:    id,
				Date:      gr.ReceiveDate,
				Note:      fmt.Sprintf("รับจาก %s (%s) lot %s exp %s", po.Supplier, gr.PoRef, item.Lot, item.ExpiryDate),
				ChangedBy: username.(string),
			}
			if err := tx.Create(&movement).Error; err != nil {
				return err
			}
		}

		// Save GR
		if err := tx.Create(&gr).Error; err != nil {
			return err
		}

		// Check if PO completed
		allCompleted := true
		for _, pi := range po.Items {
			if pi.ReceivedQty < pi.Qty {
				allCompleted = false
				break
			}
		}

		newStatus := "Partial Received"
		if allCompleted {
			newStatus = "Completed"
		}
		po.Status = newStatus
		if err := tx.Save(&po).Error; err != nil {
			return err
		}

		// Add audit trail for Goods Receive
		grAudit := models.AuditEvent{
			OwnerID:   gr.ID,
			OwnerType: "goods_receives",
			Action:    "Created",
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("รับสินค้าจาก %s", po.Supplier),
		}
		if err := tx.Create(&grAudit).Error; err != nil {
			return err
		}

		// Add audit trail for PO update
		poAudit := models.AuditEvent{
			OwnerID:   po.ID,
			OwnerType: "purchase_orders",
			Action:    newStatus,
			By:        username.(string),
			At:        getNowStr(),
			Note:      fmt.Sprintf("รับสินค้า %s", gr.ID),
		}
		if err := tx.Create(&poAudit).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").Preload("LandedCosts").Preload("AuditTrail").First(&gr, "id = ?", gr.ID)
	return c.JSON(gr)
}

func recalculateProductBOMCost(tx *gorm.DB, parentSku string) error {
	var parentProduct models.Product
	if err := tx.First(&parentProduct, "sku = ?", parentSku).Error; err != nil {
		return err
	}

	var comps []models.BundleComponent
	if err := tx.Where("bundle_sku = ?", parentSku).Find(&comps).Error; err != nil {
		return err
	}

	cost, err := calculateBOMCost(tx, comps)
	if err != nil {
		return err
	}

	parentProduct.Cost = cost

	return tx.Save(&parentProduct).Error
}
