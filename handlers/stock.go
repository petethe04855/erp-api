package handlers

import (
	"fmt"
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
		id, err := NextID(tx, "GI-2026-", &models.GoodsIssue{}, "id")
		if err != nil {
			return err
		}

		gi = models.GoodsIssue{
			ID:       id,
			SKU:      req.SKU,
			SkuName:  product.Name,
			Qty:      req.Qty,
			Reason:   req.Reason,
			Note:     req.Note,
			Date:     time.Now().Format("2006-01-02"),
			IssuedBy: username.(string),
		}

		if product.IsBundle {
			// Validate components stock
			var comps []models.BundleComponent
			if err := tx.Where("bundle_sku = ?", product.SKU).Find(&comps).Error; err == nil {
				for _, comp := range comps {
					var cp models.Product
					if err := tx.First(&cp, "sku = ?", comp.ComponentSku).Error; err != nil {
						return fmt.Errorf("Component %s not found", comp.ComponentSku)
					}
					needed := comp.Qty * req.Qty
					if cp.Stock-cp.ReservedQty < needed {
						return fmt.Errorf("Component %s stock not sufficient", comp.ComponentSku)
					}

					// Deduct stock
					cp.Stock -= needed
					if err := tx.Save(&cp).Error; err != nil {
						return err
					}

					// Create Stock Movement
					movement := models.StockMovement{
						ID:        fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), comp.ComponentSku),
						SKU:       comp.ComponentSku,
						Type:      "OUT",
						Qty:       needed,
						RefDoc:    id,
						Date:      gi.Date,
						Note:      fmt.Sprintf("Bundle GI: %s × %d → %s", product.SKU, req.Qty, comp.ComponentSku),
						ChangedBy: username.(string),
					}
					if err := tx.Create(&movement).Error; err != nil {
						return err
					}
				}
			}
		} else {
			// Regular product: validate and deduct via FEFO
			available := product.Stock - product.ReservedQty
			if req.Qty > available {
				return fmt.Errorf("Stock not sufficient for %s", product.Name)
			}

			if err := deductFefoStock(tx, req.SKU, req.Qty, id, username.(string)); err != nil {
				return err
			}
		}

		if err := tx.Create(&gi).Error; err != nil {
			return err
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
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
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
		id, err := NextID(tx, "RET-2026-", &models.StockReturn{}, "id")
		if err != nil {
			return err
		}

		sr = models.StockReturn{
			ID:         id,
			SoRef:      req.SoRef,
			SKU:        req.SKU,
			SkuName:    product.Name,
			Qty:        req.Qty,
			Condition:  req.Condition,
			Reason:     req.Reason,
			Note:       req.Note,
			Date:       time.Now().Format("2006-01-02"),
			ReturnedBy: username.(string),
			Refunded:   false,
		}

		if err := tx.Create(&sr).Error; err != nil {
			return err
		}

		// Create Stock Movement
		movement := models.StockMovement{
			ID:        fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), req.SKU),
			SKU:       req.SKU,
			Type:      "IN",
			Qty:       req.Qty,
			RefDoc:    id,
			Date:      sr.Date,
			Note:      fmt.Sprintf("รับคืน: %s", req.Reason),
			ChangedBy: username.(string),
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}

		// Increment stock if condition is good ("ดี")
		if req.Condition == "ดี" {
			product.Stock += req.Qty
			if err := tx.Save(&product).Error; err != nil {
				return err
			}
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
		id, err := NextID(tx, "ADJ-2026-", &models.StockAdjustment{}, "id")
		if err != nil {
			return err
		}

		adj = models.StockAdjustment{
			ID:        id,
			Date:      time.Now().Format("2006-01-02"),
			CheckedBy: username.(string),
			Note:      req.Note,
		}

		var items []models.StockAdjustmentItem
		for _, item := range req.Items {
			var p models.Product
			if err := tx.First(&p, "sku = ?", item.SKU).Error; err != nil {
				return err
			}

			variance := item.ActualQty - p.Stock
			adjItem := models.StockAdjustmentItem{
				AdjustmentID: id,
				SKU:          item.SKU,
				SkuName:      p.Name,
				SystemQty:    p.Stock,
				ActualQty:    item.ActualQty,
				Variance:     variance,
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
					ID:        fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), item.SKU),
					SKU:       item.SKU,
					Type:      moveType,
					Qty:       absVar,
					RefDoc:    id,
					Date:      adj.Date,
					Note:      fmt.Sprintf("ปรับสต๊อก: นับจริง %d", item.ActualQty),
					ChangedBy: username.(string),
				}
				if err := tx.Create(&movement).Error; err != nil {
					return err
				}
			}
		}

		adj.Items = items
		if err := tx.Create(&adj).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	database.DB.Preload("Items").First(&adj, "id = ?", adj.ID)
	return c.JSON(adj)
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

	if product.Stock < req.Qty {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Transfer qty exceeds stock count"})
	}

	username := c.Locals("name")
	if username == nil {
		username = "System"
	}

	var st models.StockTransfer
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		id, err := NextID(tx, "TRF-2026-", &models.StockTransfer{}, "id")
		if err != nil {
			return err
		}

		st = models.StockTransfer{
			ID:            id,
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
			ID:        fmt.Sprintf("SM-%d-%s", time.Now().UnixNano(), req.SKU),
			SKU:       req.SKU,
			Type:      "OUT",
			Qty:       req.Qty,
			RefDoc:    id,
			Date:      st.Date,
			Note:      fmt.Sprintf("โอนย้าย %s → %s", req.FromLocation, req.ToLocation),
			ChangedBy: username.(string),
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
