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
	if req.Qty <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "qty must be greater than zero"})
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

		if product.IsBundle {
			// Validate components stock
			var comps []models.BundleComponent
			if err := tx.Where("bundle_sku = ?", product.SKU).Find(&comps).Error; err == nil {
				for _, comp := range comps {
					if comp.ComponentType == "expense" {
						continue
					}
					var cp models.Product
					if err := tx.First(&cp, "sku = ?", comp.ComponentSku).Error; err != nil {
						return fmt.Errorf("Component %s not found", comp.ComponentSku)
					}
					required := comp.Qty
					if comp.Unit == "g" && cp.BaseUnit == "kg" {
						required /= 1000
					} else if comp.Unit == "kg" && cp.BaseUnit == "g" {
						required *= 1000
					}
					needed := int(math.Ceil(required * float64(req.Qty)))
					if cp.Stock-cp.ReservedQty < needed {
						return fmt.Errorf("Component %s stock not sufficient", comp.ComponentSku)
					}
					if err := deductFefoStock(tx, comp.ComponentSku, needed, gi.Code, &gi.ID, username.(string)); err != nil {
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

			if err := deductFefoStock(tx, req.SKU, req.Qty, gi.Code, &gi.ID, username.(string)); err != nil {
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

		channel := req.Channel
		var soID *uint
		if req.SoRef != "" {
			var so models.SalesOrder
			if err := tx.First(&so, "id = ? OR code = ?", req.SoRef, req.SoRef).Error; err == nil {
				soID = &so.ID
				channel = string(so.Channel)
			}
		}

		sr = models.StockReturn{
			Code:         code,
			SalesOrderID: soID,
			SoRef:        req.SoRef,
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
			Channel:      channel,
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
		if err := tx.First(&sr, "id = ? OR code = ?", id, id).Error; err != nil {
			return err
		}
		if sr.Status != "Pending" {
			return fmt.Errorf("return is already processed")
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

		var items []models.StockAdjustmentItem
		for _, item := range req.Items {
			var p models.Product
			if err := tx.First(&p, "sku = ?", item.SKU).Error; err != nil {
				return err
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
