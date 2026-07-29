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

// POST /api/goods-issues
func CreateGoodsIssue(c *fiber.Ctx) error {
	var req struct {
		SKU    string `json:"sku"`
		Qty    int    `json:"qty"`
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.SKU == "" || req.Reason == "" || req.Qty <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "sku, reason, and qty greater than zero are required"})
	}

	var product models.Product
	if err := database.DB.First(&product, "sku = ?", req.SKU).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	var gi models.GoodsIssue
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "GI-2026-", &models.GoodsIssue{}, "code")
		if err != nil {
			return err
		}

		gi = models.GoodsIssue{
			Code:      code,
			ProductID: product.ID,
			SKU:       req.SKU,
			SkuName:   product.Name,
			Qty:       req.Qty,
			Reason:    req.Reason,
			Note:      req.Note,
			Date:      time.Now().Format("2006-01-02"),
			IssuedBy:  username.(string),
		}

		if err := tx.Create(&gi).Error; err != nil {
			return err
		}

		requirements, err := expandBOMMaterialRequirements(tx, product.SKU, float64(req.Qty), map[string]bool{})
		if err != nil {
			return err
		}
		for sku, requiredQty := range requirements {
			needed := int(math.Ceil(requiredQty))
			var component models.Product
			if err := tx.First(&component, "sku = ?", sku).Error; err != nil {
				return err
			}
			if component.Stock-component.ReservedQty < needed {
				return fmt.Errorf("stock not sufficient for %s", component.Name)
			}
			if err := deductFefoStock(tx, sku, needed, gi.Code, &gi.ID, "goods_issues", username.(string)); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(gi)
}

// POST /api/stock-returns
func CreateStockReturn(c *fiber.Ctx) error {
	var req struct {
		SoRef     string `json:"soRef"`
		SKU       string `json:"sku"`
		Qty       int    `json:"qty"`
		Condition string `json:"condition"`
		Reason    string `json:"reason"`
		Note      string `json:"note"`
		Channel   string `json:"channel"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.SoRef == "" || req.SKU == "" || req.Qty <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "sales order, sku, and qty greater than zero are required"})
	}
	if req.Condition != "ดี" && req.Condition != "เสียหาย" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "condition must be ดี or เสียหาย"})
	}

	var product models.Product
	if err := database.DB.First(&product, "sku = ?", req.SKU).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	var sr models.StockReturn
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "RET-2026-", &models.StockReturn{}, "code")
		if err != nil {
			return err
		}

		var so models.SalesOrder
		if err := ByIDOrCode(tx, req.SoRef).Preload("Lines").First(&so).Error; err != nil {
			return fmt.Errorf("sales order not found")
		}
		if so.Status != "Completed" {
			return fmt.Errorf("only completed sales orders can be returned")
		}
		var soldQty int
		for _, line := range so.Lines {
			if line.SKU == req.SKU {
				soldQty += line.Qty
			}
		}
		if soldQty == 0 {
			return fmt.Errorf("product %s is not on sales order %s", req.SKU, so.Code)
		}
		var returnedQty int64
		if err := tx.Model(&models.StockReturn{}).Where("sales_order_id = ? AND sku = ? AND status <> ?", so.ID, req.SKU, "Cancelled").Select("COALESCE(SUM(qty), 0)").Scan(&returnedQty).Error; err != nil {
			return err
		}
		if int(returnedQty)+req.Qty > soldQty {
			return fmt.Errorf("return quantity exceeds quantity sold for %s", req.SKU)
		}

		sr = models.StockReturn{
			Code:         code,
			SalesOrderID: &so.ID,
			SoRef:        so.Code,
			ProductID:    product.ID,
			SKU:          req.SKU,
			SkuName:      product.Name,
			Qty:          req.Qty,
			Condition:    req.Condition,
			Reason:       req.Reason,
			Note:         req.Note,
			Date:         time.Now().Format("2006-01-02"),
			ReturnedBy:   username.(string),
			Refunded:     false,
			Channel:      so.Channel,
			Status:       "Pending",
		}

		if err := tx.Create(&sr).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(sr)
}

// PUT /api/stock-returns/:id/status
func UpdateStockReturnStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		Status string `json:"status"` // Completed, Cancelled
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	username := c.Locals("name")
	if username == nil {
		username = "System"
	}
	var sr models.StockReturn
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := ByIDOrCode(tx, id).First(&sr).Error; err != nil {
			return err
		}
		if sr.Status != "Pending" {
			return fmt.Errorf("return is already processed")
		}
		if req.Status != "Completed" && req.Status != "Cancelled" {
			return fmt.Errorf("status must be Completed or Cancelled")
		}
		sr.Status = req.Status
		if req.Status == "Completed" {
			var product models.Product
			if err := tx.First(&product, "sku = ?", sr.SKU).Error; err != nil {
				return err
			}
			if sr.Condition == "ดี" {
				product.Stock += sr.Qty
				if err := tx.Save(&product).Error; err != nil {
					return err
				}
				lot := models.StockLot{
					Code:           fmt.Sprintf("LOT-%s", sr.Code),
					ProductID:      product.ID,
					SKU:            sr.SKU,
					Lot:            fmt.Sprintf("RET-%s", sr.Code),
					Qty:            sr.Qty,
					RemainingQty:   sr.Qty,
					LandedUnitCost: product.Cost,
					ReceivedDate:   time.Now().Format("2006-01-02"),
					GrRef:          sr.Code,
				}
				if err := tx.Create(&lot).Error; err != nil {
					return err
				}
				movement := models.StockMovement{
					Code:       fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), sr.SKU),
					ProductID:  product.ID,
					SKU:        sr.SKU,
					Type:       "IN",
					Qty:        sr.Qty,
					RefDoc:     sr.Code,
					RefDocType: "stock_returns",
					RefDocID:   &sr.ID,
					Date:       time.Now().Format("2006-01-02"),
					Note:       fmt.Sprintf("รับคืน: %s - สภาพดี", sr.Reason),
					ChangedBy:  username.(string),
				}
				if err := tx.Create(&movement).Error; err != nil {
					return err
				}
			} else if sr.Condition == "เสียหาย" {
				// Do not add stock or create movements for damaged return
			}
		}
		if err := tx.Save(&sr).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(sr)
}

type StockAdjustmentRequest struct {
	Note  string `json:"note"`
	Items []struct {
		SKU       string `json:"sku"`
		ActualQty int    `json:"actualQty"`
	} `json:"items"`
}

// POST /api/stock-adjustments
func CreateStockAdjustment(c *fiber.Ctx) error {
	var req StockAdjustmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	var adj models.StockAdjustment
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "ADJ-2026-", &models.StockAdjustment{}, "code")
		if err != nil {
			return err
		}

		adj = models.StockAdjustment{
			Code:      code,
			Date:      time.Now().Format("2006-01-02"),
			CheckedBy: username.(string),
			Note:      req.Note,
		}

		if err := tx.Create(&adj).Error; err != nil {
			return err
		}
		if len(req.Items) == 0 {
			return fmt.Errorf("at least one counted item is required")
		}

		var items []models.StockAdjustmentItem
		for _, item := range req.Items {
			if item.ActualQty < 0 {
				return fmt.Errorf("actual quantity for %s cannot be negative", item.SKU)
			}
			var p models.Product
			if err := tx.First(&p, "sku = ?", item.SKU).Error; err != nil {
				return err
			}
			if item.ActualQty < p.ReservedQty {
				return fmt.Errorf("count for %s cannot be below reserved sales quantity (%d)", p.Name, p.ReservedQty)
			}

			variance := item.ActualQty - p.Stock
			adjItem := models.StockAdjustmentItem{
				StockAdjustmentID: adj.ID,
				ProductID:         p.ID,
				SKU:               item.SKU,
				SkuName:           p.Name,
				SystemQty:         p.Stock,
				ActualQty:         item.ActualQty,
				Variance:          variance,
			}
			if err := tx.Create(&adjItem).Error; err != nil {
				return err
			}
			items = append(items, adjItem)

			// Update product stock count
			p.Stock = item.ActualQty
			if err := tx.Save(&p).Error; err != nil {
				return err
			}

			if variance != 0 {
				moveType := "IN"
				if variance < 0 {
					moveType = "OUT"
				}
				absVar := variance
				if absVar < 0 {
					absVar = -absVar
				}

				// Create Stock Movement
				movement := models.StockMovement{
					Code:       fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), item.SKU),
					ProductID:  p.ID,
					SKU:        item.SKU,
					Type:       moveType,
					Qty:        absVar,
					RefDoc:     adj.Code,
					RefDocType: "stock_adjustments",
					RefDocID:   &adj.ID,
					Date:       adj.Date,
					Note:       fmt.Sprintf("ปรับสต๊อก: นับจริง %d", item.ActualQty),
					ChangedBy:  username.(string),
				}
				if err := tx.Create(&movement).Error; err != nil {
					return err
				}
				if err := reconcileAdjustmentLots(tx, p, variance, adj); err != nil {
					return err
				}
			}
		}

		adj.Items = items
		if err := tx.Save(&adj).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").First(&adj, adj.ID)
	return c.JSON(adj)
}

// reconcileAdjustmentLots keeps lot balances equal to the product balance after a physical count.
// A positive variance becomes a traceable adjustment lot; a negative variance is removed FEFO.
func reconcileAdjustmentLots(tx *gorm.DB, product models.Product, variance int, adj models.StockAdjustment) error {
	if variance > 0 {
		lot := models.StockLot{
			Code:           fmt.Sprintf("LOT-%s-%s", adj.Code, product.SKU),
			ProductID:      product.ID,
			SKU:            product.SKU,
			Lot:            adj.Code,
			Qty:            variance,
			RemainingQty:   variance,
			LandedUnitCost: product.Cost,
			ReceivedDate:   adj.Date,
			GrRef:          adj.Code,
		}
		return tx.Create(&lot).Error
	}

	remaining := -variance
	var lots []models.StockLot
	if err := tx.Where("sku = ? AND remaining_qty > 0", product.SKU).
		Order("CASE WHEN expiry_date IS NULL OR expiry_date = '' THEN 1 ELSE 0 END, expiry_date ASC").
		Find(&lots).Error; err != nil {
		return err
	}
	for i := range lots {
		if remaining == 0 {
			break
		}
		deduct := lots[i].RemainingQty
		if deduct > remaining {
			deduct = remaining
		}
		lots[i].RemainingQty -= deduct
		remaining -= deduct
		if err := tx.Save(&lots[i]).Error; err != nil {
			return err
		}
	}
	if remaining != 0 {
		return fmt.Errorf("lot stock is inconsistent for %s: missing %d units", product.SKU, remaining)
	}
	return nil
}

// POST /api/stock-transfers
func CreateStockTransfer(c *fiber.Ctx) error {
	var req struct {
		SKU          string `json:"sku"`
		Qty          int    `json:"qty"`
		FromLocation string `json:"fromLocation"`
		ToLocation   string `json:"toLocation"`
		Note         string `json:"note"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var product models.Product
	if err := database.DB.First(&product, "sku = ?", req.SKU).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	if req.Qty <= 0 || req.Qty > product.Stock-product.ReservedQty {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Transfer qty exceeds available stock"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	var st models.StockTransfer
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		code, err := NextCode(tx, "TRF-2026-", &models.StockTransfer{}, "code")
		if err != nil {
			return err
		}

		st = models.StockTransfer{
			Code:          code,
			ProductID:     product.ID,
			SKU:           req.SKU,
			SkuName:       product.Name,
			Qty:           req.Qty,
			FromLocation:  req.FromLocation,
			ToLocation:    req.ToLocation,
			Note:          req.Note,
			Date:          time.Now().Format("2006-01-02"),
			TransferredBy: username.(string),
		}

		if err := tx.Create(&st).Error; err != nil {
			return err
		}

		// Create Stock Movement
		movement := models.StockMovement{
			Code:       fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), req.SKU),
			ProductID:  product.ID,
			SKU:        req.SKU,
			Type:       "OUT",
			Qty:        req.Qty,
			RefDoc:     st.Code,
			RefDocType: "stock_transfers",
			RefDocID:   &st.ID,
			Date:       st.Date,
			Note:       fmt.Sprintf("โอนย้าย %s → %s", req.FromLocation, req.ToLocation),
			ChangedBy:  username.(string),
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(st)
}
