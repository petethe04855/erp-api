package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainCustomer "chawy-erp-api/internal/domain/customer"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainQuotation "chawy-erp-api/internal/domain/quotation"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainTikTok "chawy-erp-api/internal/domain/tiktok"
	usecaseOrder "chawy-erp-api/internal/usecase/order"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkspaceHandler struct {
	db           *gorm.DB
	orderUsecase usecaseOrder.Usecase
}

func NewWorkspaceHandler(db *gorm.DB, orderUsecase usecaseOrder.Usecase) *WorkspaceHandler {
	return &WorkspaceHandler{
		db:           db,
		orderUsecase: orderUsecase,
	}
}

// ProductRecord matching erp-web-v2 features/erp/types/records.ts
type ProductRecord struct {
	ID             uint    `json:"id"`
	SKU            string  `json:"sku"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Barcode        string  `json:"barcode"`
	BaseUnit       string  `json:"baseUnit"`
	RetailPrice    float64 `json:"retailPrice"`
	WholesalePrice float64 `json:"wholesalePrice"`
	Cost           float64 `json:"cost"`
	Stock          int     `json:"stock"`
	ReservedQty    int     `json:"reservedQty"`
	Reorder        int     `json:"reorder"`
	IsBundle       bool    `json:"isBundle"`
	IsActive       bool    `json:"isActive"`
	Available      int     `json:"available"`
	Image          string  `json:"image"`
}

func (h *WorkspaceHandler) GetProducts(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	search := strings.TrimSpace(c.Query("search", ""))

	var skus []domainSKU.SKU
	var total int64
	query := h.db.WithContext(c.Context()).Model(&domainSKU.SKU{})
	if search != "" {
		s := "%" + search + "%"
		query = query.Where("sku ILIKE ? OR name ILIKE ? OR barcode ILIKE ?", s, s, s)
	}
	query.Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&skus)

	// Fetch stocks for these SKUs
	records := make([]ProductRecord, len(skus))
	for i, s := range skus {
		var stk domainStock.Stock
		_ = h.db.WithContext(c.Context()).Where("sku_id = ?", s.ID).First(&stk).Error

		pType := "Finished Product"
		if s.Category != "" {
			pType = s.Category
		}

		// Calculate TikTok Shop pending reserved quantity for this SKU
		// Deduplicate candidate SKUs to avoid counting reserved qty twice when TikTok SKU equals ERP SKU
		skuSet := make(map[string]bool)
		skuUpper := strings.ToUpper(strings.TrimSpace(s.SKU))
		if skuUpper != "" {
			skuSet[skuUpper] = true
		}

		var mappings []domainTikTok.SKUMapping
		_ = h.db.WithContext(c.Context()).Where("UPPER(TRIM(erp_sku)) = ?", skuUpper).Find(&mappings).Error
		for _, m := range mappings {
			ttUpper := strings.ToUpper(strings.TrimSpace(m.TikTokSKU))
			if ttUpper != "" {
				skuSet[ttUpper] = true
			}
		}

		var candidateSKUs []string
		for k := range skuSet {
			candidateSKUs = append(candidateSKUs, k)
		}

		var tiktokReserved int64
		if len(candidateSKUs) > 0 {
			_ = h.db.WithContext(c.Context()).Model(&domainTikTok.TiktokOrderItem{}).
				Joins("JOIN tiktok_orders ON tiktok_orders.id = tiktok_order_items.order_id").
				Where("UPPER(TRIM(tiktok_order_items.sku)) IN ? AND tiktok_orders.stock_deducted = false AND UPPER(tiktok_orders.status) NOT IN ('CANCELLED', 'CANCELED', 'COMPLETED', 'DELIVERED', 'SHIPPED')", candidateSKUs).
				Select("COALESCE(SUM(tiktok_order_items.qty), 0)").
				Scan(&tiktokReserved).Error
		}

		totalReserved := stk.ReservedQty + int(tiktokReserved)
		available := stk.Quantity - totalReserved
		if available < 0 {
			available = 0
		}

		records[i] = ProductRecord{
			ID:             s.ID,
			SKU:            s.SKU,
			Name:           s.Name,
			Type:           pType,
			Barcode:        s.Barcode,
			BaseUnit:       "ชิ้น",
			RetailPrice:    s.Price,
			WholesalePrice: s.Price,
			Cost:           s.CostPrice,
			Stock:          stk.Quantity,
			ReservedQty:    totalReserved,
			Reorder:        10,
			IsBundle:       s.IsBundle,
			IsActive:       s.Status == "active",
			Available:      available,
			Image:          s.Image,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) GetProductByCode(c *fiber.Ctx) error {
	code := c.Params("code")
	var s domainSKU.SKU
	if err := h.db.WithContext(c.Context()).Where("sku = ?", code).First(&s).Error; err != nil {
		return response.NotFound(c, "Product not found")
	}

	var stk domainStock.Stock
	_ = h.db.WithContext(c.Context()).Where("sku_id = ?", s.ID).First(&stk).Error

	pType := "Finished Product"
	if s.Category != "" {
		pType = s.Category
	}

	skuSet := make(map[string]bool)
	skuUpper := strings.ToUpper(strings.TrimSpace(s.SKU))
	if skuUpper != "" {
		skuSet[skuUpper] = true
	}

	var mappings []domainTikTok.SKUMapping
	_ = h.db.WithContext(c.Context()).Where("UPPER(TRIM(erp_sku)) = ?", skuUpper).Find(&mappings).Error
	for _, m := range mappings {
		ttUpper := strings.ToUpper(strings.TrimSpace(m.TikTokSKU))
		if ttUpper != "" {
			skuSet[ttUpper] = true
		}
	}

	var candidateSKUs []string
	for k := range skuSet {
		candidateSKUs = append(candidateSKUs, k)
	}

	var tiktokReserved int64
	if len(candidateSKUs) > 0 {
		_ = h.db.WithContext(c.Context()).Model(&domainTikTok.TiktokOrderItem{}).
			Joins("JOIN tiktok_orders ON tiktok_orders.id = tiktok_order_items.order_id").
			Where("UPPER(TRIM(tiktok_order_items.sku)) IN ? AND tiktok_orders.stock_deducted = false AND UPPER(tiktok_orders.status) NOT IN ('CANCELLED', 'CANCELED', 'COMPLETED', 'DELIVERED', 'SHIPPED')", candidateSKUs).
			Select("COALESCE(SUM(tiktok_order_items.qty), 0)").
			Scan(&tiktokReserved).Error
	}

	totalReserved := stk.ReservedQty + int(tiktokReserved)
	available := stk.Quantity - totalReserved
	if available < 0 {
		available = 0
	}

	rec := ProductRecord{
		ID:             s.ID,
		SKU:            s.SKU,
		Name:           s.Name,
		Type:           pType,
		Barcode:        s.Barcode,
		BaseUnit:       "ชิ้น",
		RetailPrice:    s.Price,
		WholesalePrice: s.Price,
		Cost:           s.CostPrice,
		Stock:          stk.Quantity,
		ReservedQty:    totalReserved,
		Reorder:        10,
		IsBundle:       s.IsBundle,
		IsActive:       s.Status == "active",
		Available:      available,
		Image:          s.Image,
	}
	return response.OK(c, rec)
}

func (h *WorkspaceHandler) CreateProduct(c *fiber.Ctx) error {
	var req struct {
		SKU         string  `json:"sku"`
		Name        string  `json:"name"`
		Type        string  `json:"type"`
		BaseUnit    string  `json:"baseUnit"`
		RetailPrice float64 `json:"retailPrice"`
		Cost        float64 `json:"cost"`
		IsBundle    bool    `json:"isBundle"`
		Image       string  `json:"image"`
		Components  []struct {
			ComponentSKU string `json:"componentSku"`
			SKU          string `json:"sku"`
			Quantity     int    `json:"quantity"`
			Qty          int    `json:"qty"`
			Note         string `json:"note"`
		} `json:"components"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	skuCode := strings.ToUpper(strings.TrimSpace(req.SKU))
	if skuCode == "" {
		return response.BadRequest(c, "SKU is required")
	}
	if req.Name == "" {
		return response.BadRequest(c, "Product name is required")
	}

	if req.IsBundle {
		if len(req.Components) == 0 {
			return response.BadRequest(c, "Bundle product must include at least one component")
		}
		for _, comp := range req.Components {
			cSKU := comp.ComponentSKU
			if cSKU == "" && comp.SKU != "" {
				cSKU = comp.SKU
			}
			cSKU = strings.ToUpper(strings.TrimSpace(cSKU))
			if cSKU == "" {
				return response.BadRequest(c, "Component SKU cannot be empty")
			}
			qty := comp.Quantity
			if qty <= 0 && comp.Qty > 0 {
				qty = comp.Qty
			}
			if qty <= 0 {
				return response.BadRequest(c, fmt.Sprintf("Component quantity for SKU %s must be greater than 0", cSKU))
			}
		}
	}

	var skuEntity domainSKU.SKU
	err := h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		skuEntity = domainSKU.SKU{
			SKU:       skuCode,
			Name:      req.Name,
			Category:  req.Type,
			Price:     req.RetailPrice,
			CostPrice: req.Cost,
			IsBundle:  req.IsBundle,
			Image:     req.Image,
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := tx.Create(&skuEntity).Error; err != nil {
			return err
		}

		// Create initial stock row
		stockEntity := domainStock.Stock{
			SKUID:        skuEntity.ID,
			SKUCode:      skuEntity.SKU,
			WarehouseID:  1,
			Quantity:     0,
			AvailableQty: 0,
			UpdatedAt:    time.Now(),
		}
		if err := tx.Create(&stockEntity).Error; err != nil {
			return err
		}

		if req.IsBundle {
			for _, comp := range req.Components {
				compSKU := comp.ComponentSKU
				if compSKU == "" && comp.SKU != "" {
					compSKU = comp.SKU
				}
				compSKU = strings.ToUpper(strings.TrimSpace(compSKU))
				if compSKU == "" {
					continue
				}

				qty := comp.Quantity
				if qty <= 0 && comp.Qty > 0 {
					qty = comp.Qty
				}
				if qty <= 0 {
					qty = 1
				}

				bundleItem := domainBundle.BundleItem{
					BundleSKU:    skuEntity.SKU,
					ComponentSKU: compSKU,
					Quantity:     qty,
					Note:         comp.Note,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				if err := tx.Create(&bundleItem).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return response.BadRequest(c, "Failed to create product: "+err.Error())
	}

	rec := ProductRecord{
		ID:          skuEntity.ID,
		SKU:         skuEntity.SKU,
		Name:        skuEntity.Name,
		Type:        skuEntity.Category,
		BaseUnit:    req.BaseUnit,
		RetailPrice: skuEntity.Price,
		Cost:        skuEntity.CostPrice,
		IsBundle:    skuEntity.IsBundle,
		IsActive:    true,
		Image:       skuEntity.Image,
	}
	return response.Created(c, rec, "Product created successfully")
}

func (h *WorkspaceHandler) UpdateProductStatus(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.Params("code"))
	if code == "" {
		return response.BadRequest(c, "Product SKU is required")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var s domainSKU.SKU
	query := h.db.WithContext(c.Context())
	if id, err := strconv.ParseUint(code, 10, 32); err == nil {
		// FULL-12: numeric-looking codes match ID only, never a SKU string,
		// so /products/42 can never hit a different product whose SKU is "42".
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("sku = ?", code)
	}

	if err := query.First(&s).Error; err != nil {
		return response.NotFound(c, "Product not found")
	}

	newStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if newStatus != "active" && newStatus != "inactive" {
		if newStatus == "archived" {
			newStatus = "inactive"
		} else {
			newStatus = "active"
		}
	}

	s.Status = newStatus
	s.UpdatedAt = time.Now()
	if err := h.db.WithContext(c.Context()).Save(&s).Error; err != nil {
		return response.InternalServerError(c, "Failed to update product status")
	}

	return response.OK(c, fiber.Map{
		"sku":    s.SKU,
		"status": s.Status,
	}, "Product status updated successfully")
}

func (h *WorkspaceHandler) UpdateProduct(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.Params("code"))
	if code == "" {
		return response.BadRequest(c, "Product SKU is required")
	}

	var req struct {
		Name        *string  `json:"name"`
		Type        *string  `json:"type"`
		RetailPrice *float64 `json:"retailPrice"`
		Cost        *float64 `json:"cost"`
		IsBundle    *bool    `json:"isBundle"`
		Image       *string  `json:"image"`
		Status      *string  `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var s domainSKU.SKU
	query := h.db.WithContext(c.Context())
	if id, err := strconv.ParseUint(code, 10, 32); err == nil {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("sku = ?", code)
	}

	if err := query.First(&s).Error; err != nil {
		return response.NotFound(c, "Product not found")
	}

	// FULL-11: only fields present in the payload are applied; omitted fields
	// keep their stored values (cost, bundle flag, image survive partial updates).
	if req.Name != nil && *req.Name != "" {
		s.Name = *req.Name
	}
	if req.Type != nil {
		s.Category = *req.Type
	}
	if req.RetailPrice != nil && *req.RetailPrice > 0 {
		s.Price = *req.RetailPrice
	}
	if req.Cost != nil && *req.Cost >= 0 {
		s.CostPrice = *req.Cost
	}
	if req.IsBundle != nil {
		s.IsBundle = *req.IsBundle
	}
	if req.Image != nil {
		s.Image = *req.Image
	}
	if req.Status != nil && *req.Status != "" {
		s.Status = strings.ToLower(strings.TrimSpace(*req.Status))
	}
	s.UpdatedAt = time.Now()

	if err := h.db.WithContext(c.Context()).Save(&s).Error; err != nil {
		return response.InternalServerError(c, "Failed to update product: "+err.Error())
	}

	rec := ProductRecord{
		ID:          s.ID,
		SKU:         s.SKU,
		Name:        s.Name,
		Type:        s.Category,
		RetailPrice: s.Price,
		Cost:        s.CostPrice,
		IsBundle:    s.IsBundle,
		IsActive:    s.Status == "active",
		Image:       s.Image,
	}
	return response.OK(c, rec, "Product updated successfully")
}

func (h *WorkspaceHandler) DeleteProduct(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.Params("code"))
	if code == "" {
		return response.BadRequest(c, "Product SKU is required")
	}

	var s domainSKU.SKU
	query := h.db.WithContext(c.Context())
	if id, err := strconv.ParseUint(code, 10, 32); err == nil {
		query = query.Where("id = ?", id)
	} else {
		query = query.Where("sku = ?", code)
	}

	if err := query.First(&s).Error; err != nil {
		return response.NotFound(c, "Product not found")
	}

	// FULL-13: reference check + delete run in ONE transaction; the reference
	// query error is no longer discarded. Concurrent reference creation after
	// the check is still possible until FK restrictions exist, but the check
	// itself can no longer fail-open.
	var inUse bool
	err := h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		// Re-lock the SKU row inside the tx so the product cannot be deleted
		// twice or mutated while we inspect references.
		var locked domainSKU.SKU
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, s.ID).Error; err != nil {
			return err
		}

		type refCount struct {
			Stocks      int64 `gorm:"column:stocks"`
			Movements   int64 `gorm:"column:movements"`
			OrderItems  int64 `gorm:"column:order_items"`
			POItems     int64 `gorm:"column:po_items"`
			QuotationLn int64 `gorm:"column:quotation_lines"`
			BundleComps int64 `gorm:"column:bundle_components"`
		}
		var refs refCount
		if err := tx.Raw(`
			SELECT
				(SELECT COUNT(*) FROM stocks WHERE sku_id = ?) AS stocks,
				(SELECT COUNT(*) FROM stock_movements WHERE sku_id = ?) AS movements,
				(SELECT COUNT(*) FROM order_items WHERE sku = ?) AS order_items,
				(SELECT COUNT(*) FROM po_items WHERE sku = ?) AS po_items,
				(SELECT COUNT(*) FROM quotation_lines WHERE sku = ?) AS quotation_lines,
				(SELECT COUNT(*) FROM bundle_items WHERE component_sku = ? OR bundle_sku = ?) AS bundle_components
		`, s.ID, s.ID, s.SKU, s.SKU, s.SKU, s.SKU, s.SKU).Scan(&refs).Error; err != nil {
			return fmt.Errorf("reference check failed: %w", err)
		}

		if refs.Stocks > 0 || refs.Movements > 0 || refs.OrderItems > 0 || refs.POItems > 0 || refs.QuotationLn > 0 || refs.BundleComps > 0 {
			inUse = true
			return nil
		}

		return tx.Delete(&domainSKU.SKU{}, s.ID).Error
	})

	if err != nil {
		return response.InternalServerError(c, "Failed to delete product: "+err.Error())
	}
	if inUse {
		return response.BadRequest(c, "Cannot delete product: it is referenced by stock, movement history, orders, POs, quotations or bundle formulas. Archive it instead.", "PRODUCT_IN_USE")
	}

	return response.OK(c, nil, "Product deleted successfully")
}

// OrderRecord matching erp-web-v2 features/erp/types/records.ts
type OrderRecord struct {
	ID       uint    `json:"id"`
	Code     string  `json:"code"`
	Customer string  `json:"customer"`
	Date     string  `json:"date"`
	Amount   float64 `json:"amount"`
	Status   string  `json:"status"`
	Channel  string  `json:"channel"`
	Items    int     `json:"items"`
	InvRef   string  `json:"invRef"`
}

func (h *WorkspaceHandler) GetOrders(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	var orders []domainOrder.Order
	var total int64
	query := h.db.WithContext(c.Context()).Model(&domainOrder.Order{})

	if status := c.Query("status", ""); status != "" && !strings.EqualFold(status, "all") {
		stUpper := strings.ToUpper(strings.TrimSpace(status))
		if stUpper == "COMPLETED" {
			query = query.Where("UPPER(status) IN ('COMPLETED', 'SHIPPED')")
		} else {
			query = query.Where("UPPER(status) = ?", stUpper)
		}
	}

	if channel := c.Query("channel", ""); channel != "" && !strings.EqualFold(channel, "all") {
		cLower := strings.ToLower(channel)
		if cLower == "manual" {
			query = query.Where("LOWER(channel) IN (?) OR channel IS NULL OR channel = ''", []string{"manual", "direct"})
		} else {
			query = query.Where("LOWER(channel) LIKE ?", "%"+cLower+"%")
		}
	}

	paymentStatus := c.Query("paymentStatus", "")
	if strings.EqualFold(paymentStatus, "paid") {
		// Paid orders have an invoice with status PAID
		query = query.Where("id IN (SELECT order_id FROM invoices WHERE order_id IS NOT NULL AND status = 'PAID')")
	} else if strings.EqualFold(paymentStatus, "unpaid") {
		// Unpaid orders have either no invoice or an unpaid invoice
		query = query.Where("id NOT IN (SELECT order_id FROM invoices WHERE order_id IS NOT NULL AND status = 'PAID')")
	}

	query.Count(&total)
	query.Preload("Items").Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&orders)

	// Preload invoice map to populate invRef
	var invoices []domainInvoice.Invoice
	orderIDs := make([]uint, len(orders))
	for i, o := range orders {
		orderIDs[i] = o.ID
	}
	invoiceMap := make(map[uint]domainInvoice.Invoice)
	if len(orderIDs) > 0 {
		h.db.WithContext(c.Context()).Where("order_id IN ?", orderIDs).Find(&invoices)
		for _, inv := range invoices {
			if inv.OrderID != nil {
				invoiceMap[*inv.OrderID] = inv
			}
		}
	}

	records := make([]OrderRecord, len(orders))
	for i, o := range orders {
		invRef := "—"
		if inv, ok := invoiceMap[o.ID]; ok {
			invRef = string(inv.Status)
		}

		records[i] = OrderRecord{
			ID:       o.ID,
			Code:     o.OrderNo,
			Customer: o.CustomerName,
			Date:     o.CreatedAt.Format("2006-01-02"),
			Amount:   o.TotalAmount,
			Status:   string(o.Status),
			Channel:  o.Channel,
			Items:    len(o.Items),
			InvRef:   invRef,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) CreateSalesOrder(c *fiber.Ctx) error {
	var req struct {
		Customer   string `json:"customer"`
		Channel    string `json:"channel"`
		IncludeVat *bool  `json:"includeVat"`
		Lines      []struct {
			SKU       string  `json:"sku"`
			Qty       int     `json:"qty"`
			UnitPrice float64 `json:"unitPrice"`
		} `json:"lines"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var items []usecaseOrder.CreateItemInput
	for _, l := range req.Lines {
		if l.Qty <= 0 {
			return response.BadRequest(c, fmt.Sprintf("Quantity for SKU %s must be greater than 0", l.SKU))
		}
		items = append(items, usecaseOrder.CreateItemInput{
			SKU:      l.SKU,
			Quantity: l.Qty,
			Price:    l.UnitPrice,
		})
	}

	channel := req.Channel
	if channel == "" {
		channel = "Manual"
	}

	includeVat := true
	if req.IncludeVat != nil {
		includeVat = *req.IncludeVat
	}

	// FULL-17: VAT policy travels with the order; totals are computed once
	// by the Usecase. The handler never overwrites amounts afterwards.
	note := "VAT_INC:true"
	if !includeVat {
		note = "VAT_INC:false"
	}

	order, err := h.orderUsecase.Create(c.Context(), usecaseOrder.CreateOrderInput{
		CustomerName: req.Customer,
		Channel:      channel,
		Note:         note,
		Items:        items,
		VATIncluded:  includeVat,
	})
	if err != nil {
		return err
	}

	rec := OrderRecord{
		ID:       order.ID,
		Code:     order.OrderNo,
		Customer: order.CustomerName,
		Date:     order.CreatedAt.Format("2006-01-02"),
		Amount:   order.TotalAmount,
		Status:   string(order.Status),
		Channel:  order.Channel,
		Items:    len(order.Items),
		InvRef:   "—",
	}
	return response.Created(c, rec, "Sales order created successfully")
}

func (h *WorkspaceHandler) GetSalesOrderByID(c *fiber.Ctx) error {
	param := c.Params("id")
	var ord domainOrder.Order
	query := h.db.WithContext(c.Context()).Preload("Items")
	if id, err := strconv.ParseUint(param, 10, 32); err == nil {
		if err := query.Where("id = ? OR order_no = ?", id, param).First(&ord).Error; err != nil {
			return response.NotFound(c, "Sales order not found")
		}
	} else {
		if err := query.Where("order_no = ?", param).First(&ord).Error; err != nil {
			return response.NotFound(c, "Sales order not found")
		}
	}

	type LineItem struct {
		SKU       string  `json:"sku"`
		Name      string  `json:"name"`
		Quantity  int     `json:"quantity"`
		UnitPrice float64 `json:"unitPrice"`
		Subtotal  float64 `json:"subtotal"`
	}

	lines := make([]LineItem, len(ord.Items))
	for i, item := range ord.Items {
		lines[i] = LineItem{
			SKU:       item.SKU,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.Price,
			Subtotal:  item.Subtotal,
		}
	}

	includeVat := !strings.Contains(ord.Note, "VAT_INC:false")

	result := fiber.Map{
		"id":         ord.ID,
		"code":       ord.OrderNo,
		"customer":   ord.CustomerName,
		"channel":    ord.Channel,
		"date":       ord.CreatedAt.Format("2006-01-02 15:04"),
		"amount":     ord.TotalAmount,
		"status":     string(ord.Status),
		"note":       ord.Note,
		"includeVat": includeVat,
		"lines":      lines,
		"itemsCount": len(ord.Items),
	}

	return response.OK(c, result)
}

func (h *WorkspaceHandler) UpdateSalesOrderStatus(c *fiber.Ctx) error {
	param := c.Params("id")
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var ord domainOrder.Order
	if id, err := strconv.ParseUint(param, 10, 32); err == nil {
		if err := h.db.WithContext(c.Context()).Where("id = ? OR order_no = ?", id, param).First(&ord).Error; err != nil {
			return response.NotFound(c, "Sales order not found")
		}
	} else {
		if err := h.db.WithContext(c.Context()).Where("order_no = ?", param).First(&ord).Error; err != nil {
			return response.NotFound(c, "Sales order not found")
		}
	}

	targetStatus := strings.ToUpper(strings.TrimSpace(req.Status))

	// Guard: Cannot mutate already cancelled orders
	if ord.Status == domainOrder.StatusCancelled {
		return response.BadRequest(c, "Cannot change status of a cancelled order")
	}

	// Guard: If already shipped, cannot revert to pending/confirmed or cancel directly without return workflow
	if ord.Status == domainOrder.StatusShipped {
		if targetStatus == "CANCELLED" {
			return response.BadRequest(c, "Cannot cancel an order that has already been shipped")
		}
		if targetStatus == "PENDING" || targetStatus == "CONFIRMED" {
			return response.BadRequest(c, "Cannot revert an already shipped order back to pending")
		}
		if targetStatus == "COMPLETED" || targetStatus == "SHIPPED" {
			// Idempotent: already shipped/completed
			return response.OK(c, ord, "Sales order is already shipped/completed")
		}
	}

	// Target: Cancel order
	if targetStatus == "CANCELLED" {
		cancelledOrder, err := h.orderUsecase.CancelOrder(c.Context(), ord.ID)
		if err != nil {
			return response.BadRequest(c, err.Error())
		}
		return response.OK(c, cancelledOrder, "Sales order cancelled successfully")
	}

	// Target: Ship / Complete order
	if targetStatus == "COMPLETED" || targetStatus == "SHIPPED" {
		shippedOrder, err := h.orderUsecase.ShipOrder(c.Context(), ord.ID, 1)
		if err != nil {
			return response.BadRequest(c, fmt.Sprintf("ไม่สามารถตัดสต็อกได้: %s", err.Error()))
		}
		return response.OK(c, shippedOrder, "Sales order completed and stock deducted successfully")
	}

	// Whitelist allowed remaining transitions
	validStatuses := map[string]domainOrder.Status{
		"PENDING":   domainOrder.StatusPending,
		"CONFIRMED": domainOrder.StatusConfirmed,
	}
	newStatus, ok := validStatuses[targetStatus]
	if !ok {
		return response.BadRequest(c, fmt.Sprintf("Invalid order status: %s", targetStatus))
	}

	ord.Status = newStatus
	if req.Note != "" {
		ord.Note = req.Note
	}
	if err := h.db.WithContext(c.Context()).Save(&ord).Error; err != nil {
		return response.InternalServerError(c, "Failed to update order status")
	}

	return response.OK(c, ord, "Sales order status updated")
}

// InvoiceRecord matching erp-web-v2 features/erp/types/records.ts
type InvoiceRecord struct {
	ID        uint    `json:"id"`
	Code      string  `json:"code"`
	SORef     string  `json:"soRef"`
	Customer  string  `json:"customer"`
	IssueDate string  `json:"issueDate"`
	DueDate   string  `json:"dueDate"`
	Subtotal  float64 `json:"subtotal"`
	VATAmount float64 `json:"vatAmount"`
	Amount    float64 `json:"amount"`
	Paid      float64 `json:"paid"`
	Status    string  `json:"status"`
}

func (h *WorkspaceHandler) GetInvoices(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	var invoices []domainInvoice.Invoice
	var total int64
	query := h.db.WithContext(c.Context()).Model(&domainInvoice.Invoice{})
	query.Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&invoices)

	records := make([]InvoiceRecord, len(invoices))
	for i, inv := range invoices {
		paid := inv.PaidAmount
		if inv.Status == domainInvoice.StatusPaid && paid <= 0 {
			paid = inv.Amount
		}
		dueDateStr := ""
		if inv.DueDate != nil {
			dueDateStr = inv.DueDate.Format("2006-01-02")
		}
		soRef := inv.OrderNo
		if soRef == "" && inv.OrderID != nil {
			soRef = fmt.Sprintf("ORD-%04d", *inv.OrderID)
		}

		records[i] = InvoiceRecord{
			ID:        inv.ID,
			Code:      inv.InvoiceNo,
			SORef:     soRef,
			Customer:  inv.CustomerName,
			IssueDate: inv.CreatedAt.Format("2006-01-02"),
			DueDate:   dueDateStr,
			Subtotal:  inv.Amount / 1.07,
			VATAmount: inv.Amount - (inv.Amount / 1.07),
			Amount:    inv.Amount,
			Paid:      paid,
			Status:    string(inv.Status),
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) CreateInvoiceFromSO(c *fiber.Ctx) error {
	soRef := c.Params("soRef")
	var order domainOrder.Order
	err := h.db.WithContext(c.Context()).Where("order_no = ?", soRef).First(&order).Error
	if err != nil {
		// Try by ID
		orderID, _ := strconv.Atoi(soRef)
		if err := h.db.WithContext(c.Context()).First(&order, orderID).Error; err != nil {
			return response.NotFound(c, "Sales order not found")
		}
	}

	orderID := order.ID
	dueDate := time.Now().AddDate(0, 0, 30)

	// Idempotency check: return existing invoice if one already exists for this order
	var existingInv domainInvoice.Invoice
	if err := h.db.WithContext(c.Context()).Where("order_id = ? OR order_no = ?", orderID, order.OrderNo).First(&existingInv).Error; err == nil {
		dueDateStr := ""
		if existingInv.DueDate != nil {
			dueDateStr = existingInv.DueDate.Format("2006-01-02")
		}
		paid := existingInv.PaidAmount
		if existingInv.Status == domainInvoice.StatusPaid && paid <= 0 {
			paid = existingInv.Amount
		}
		return response.OK(c, InvoiceRecord{
			ID:        existingInv.ID,
			Code:      existingInv.InvoiceNo,
			SORef:     order.OrderNo,
			Customer:  existingInv.CustomerName,
			IssueDate: existingInv.CreatedAt.Format("2006-01-02"),
			DueDate:   dueDateStr,
			Subtotal:  existingInv.Amount / 1.07,
			VATAmount: existingInv.Amount - (existingInv.Amount / 1.07),
			Amount:    existingInv.Amount,
			Paid:      paid,
			Status:    string(existingInv.Status),
		}, "Existing invoice retrieved")
	}

	inv := domainInvoice.Invoice{
		InvoiceNo:    fmt.Sprintf("INV-%s", time.Now().Format("20060102150405")),
		OrderID:      &orderID,
		OrderNo:      order.OrderNo,
		CustomerID:   order.CustomerID,
		CustomerName: order.CustomerName,
		Amount:       order.TotalAmount,
		PaidAmount:   0,
		Status:       domainInvoice.StatusUnpaid,
		DueDate:      &dueDate,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.WithContext(c.Context()).Create(&inv).Error; err != nil {
		return err
	}

	rec := InvoiceRecord{
		ID:        inv.ID,
		Code:      inv.InvoiceNo,
		SORef:     order.OrderNo,
		Customer:  inv.CustomerName,
		IssueDate: inv.CreatedAt.Format("2006-01-02"),
		DueDate:   dueDate.Format("2006-01-02"),
		Subtotal:  inv.Amount / 1.07,
		VATAmount: inv.Amount - (inv.Amount / 1.07),
		Amount:    inv.Amount,
		Paid:      0,
		Status:    string(inv.Status),
	}
	return response.Created(c, rec, "Invoice created successfully")
}

func (h *WorkspaceHandler) GetInvoiceByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid invoice ID")
	}

	var inv domainInvoice.Invoice
	if err := h.db.WithContext(c.Context()).First(&inv, id).Error; err != nil {
		return response.NotFound(c, "Invoice not found")
	}

	var order domainOrder.Order
	var lines []fiber.Map
	var customerAddress string

	if inv.OrderID != nil && *inv.OrderID > 0 {
		if err := h.db.WithContext(c.Context()).Preload("Items").First(&order, *inv.OrderID).Error; err == nil {
			for _, it := range order.Items {
				lines = append(lines, fiber.Map{
					"sku":       it.SKU,
					"name":      it.Name,
					"qty":       it.Quantity,
					"price":     it.Price,
					"unitPrice": it.Price,
					"subtotal":  it.Subtotal,
				})
			}
			if order.CustomerID > 0 {
				var cust domainCustomer.Customer
				if err := h.db.WithContext(c.Context()).First(&cust, order.CustomerID).Error; err == nil {
					customerAddress = cust.Address
				}
			}
		}
	}

	dueDateStr := ""
	if inv.DueDate != nil {
		dueDateStr = inv.DueDate.Format("2006-01-02")
	}

	// FULL-19: report the real paid amount for partial payments, consistent
	// with the invoice list endpoint.
	paidAmount := inv.PaidAmount
	if inv.Status == domainInvoice.StatusPaid && paidAmount <= 0 {
		paidAmount = inv.Amount
	}

	return response.OK(c, fiber.Map{
		"id":              inv.ID,
		"code":            inv.InvoiceNo,
		"invoiceNo":       inv.InvoiceNo,
		"soRef":           inv.OrderNo,
		"orderNo":         inv.OrderNo,
		"orderId":         inv.OrderID,
		"customer":        inv.CustomerName,
		"customerName":    inv.CustomerName,
		"customerAddress": customerAddress,
		"amount":          inv.Amount,
		"totalAmount":     inv.Amount,
		"paid":            paidAmount,
		"status":          string(inv.Status),
		"paymentMethod":   inv.PaymentMethod,
		"issueDate":       inv.CreatedAt.Format("2006-01-02"),
		"date":            inv.CreatedAt.Format("2006-01-02 15:04"),
		"dueDate":         dueDateStr,
		"lines":           lines,
		"itemsCount":      len(lines),
	})
}

// CustomerRecord matching erp-web-v2 features/erp/types/records.ts
type CustomerRecord struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ContactPerson string `json:"contactPerson"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	TaxID         string `json:"taxId"`
	Address       string `json:"address"`
}

func (h *WorkspaceHandler) GetCustomers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	var customers []domainCustomer.Customer
	var total int64
	query := h.db.WithContext(c.Context()).Model(&domainCustomer.Customer{})
	query.Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&customers)

	records := make([]CustomerRecord, len(customers))
	for i, cust := range customers {
		cp := cust.ContactPerson
		if cp == "" {
			cp = cust.Name
		}
		records[i] = CustomerRecord{
			ID:            strconv.Itoa(int(cust.ID)),
			Name:          cust.Name,
			ContactPerson: cp,
			Email:         cust.Email,
			Phone:         cust.Phone,
			TaxID:         cust.TaxID,
			Address:       cust.Address,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) CreateCustomer(c *fiber.Ctx) error {
	var req struct {
		Name          string `json:"name"`
		ContactPerson string `json:"contactPerson"`
		Email         string `json:"email"`
		Phone         string `json:"phone"`
		TaxID         string `json:"taxId"`
		Address       string `json:"address"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	cust := domainCustomer.Customer{
		Code:          fmt.Sprintf("CUST-%s", time.Now().Format("20060102150405")),
		Name:          req.Name,
		ContactPerson: req.ContactPerson,
		Email:         req.Email,
		Phone:         req.Phone,
		TaxID:         req.TaxID,
		Address:       req.Address,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := h.db.WithContext(c.Context()).Create(&cust).Error; err != nil {
		return err
	}

	rec := CustomerRecord{
		ID:            strconv.Itoa(int(cust.ID)),
		Name:          cust.Name,
		ContactPerson: cust.ContactPerson,
		Email:         cust.Email,
		Phone:         cust.Phone,
		TaxID:         cust.TaxID,
		Address:       cust.Address,
	}
	return response.Created(c, rec, "Customer created successfully")
}

func (h *WorkspaceHandler) GetCustomerByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}

	var cust domainCustomer.Customer
	if err := h.db.WithContext(c.Context()).First(&cust, id).Error; err != nil {
		return response.NotFound(c, "Customer not found")
	}

	cp := cust.ContactPerson
	if cp == "" {
		cp = cust.Name
	}

	return response.OK(c, fiber.Map{
		"id":            cust.ID,
		"code":          cust.Code,
		"name":          cust.Name,
		"contactPerson": cp,
		"phone":         cust.Phone,
		"email":         cust.Email,
		"address":       cust.Address,
		"taxId":         cust.TaxID,
		"channel":       cust.Channel,
		"status":        cust.Status,
		"createdAt":     cust.CreatedAt.Format("2006-01-02 15:04"),
	})
}

func (h *WorkspaceHandler) UpdateCustomerStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid customer ID")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var cust domainCustomer.Customer
	if err := h.db.WithContext(c.Context()).First(&cust, id).Error; err != nil {
		return response.NotFound(c, "Customer not found")
	}

	cust.Status = req.Status
	cust.UpdatedAt = time.Now()
	if err := h.db.WithContext(c.Context()).Save(&cust).Error; err != nil {
		return response.InternalServerError(c, "Failed to update customer status")
	}

	return response.OK(c, cust, "Customer status updated")
}

// PurchaseRecord matching erp-web-v2 features/erp/types/records.ts
type PurchaseRecord struct {
	ID        uint    `json:"id"`
	Code      string  `json:"code"`
	Supplier  string  `json:"supplier"`
	Date      string  `json:"date"`
	ETADate   string  `json:"etaDate"`
	TotalCost float64 `json:"totalCost"`
	Status    string  `json:"status"`
}

func (h *WorkspaceHandler) GetPurchaseOrders(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}

	var pos []domainPurchasing.PurchaseOrder
	var total int64
	query := h.db.WithContext(c.Context()).Model(&domainPurchasing.PurchaseOrder{})
	query.Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&pos)

	records := make([]PurchaseRecord, len(pos))
	for i, po := range pos {
		supName := "Unknown Supplier"
		if po.SupplierName != "" {
			supName = po.SupplierName
		}
		etaDate := po.ETADate
		if etaDate == "" {
			etaDate = po.CreatedAt.AddDate(0, 0, 7).Format("2006-01-02")
		}
		records[i] = PurchaseRecord{
			ID:        po.ID,
			Code:      po.PONo,
			Supplier:  supName,
			Date:      po.CreatedAt.Format("2006-01-02"),
			ETADate:   etaDate,
			TotalCost: po.TotalAmount,
			Status:    string(po.Status),
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) GetPurchaseOrderByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid purchase order ID")
	}

	var po domainPurchasing.PurchaseOrder
	if err := h.db.WithContext(c.Context()).Preload("Items").First(&po, id).Error; err != nil {
		return response.NotFound(c, "Purchase order not found")
	}

	items := make([]fiber.Map, len(po.Items))
	for i, item := range po.Items {
		items[i] = fiber.Map{
			"id":          item.ID,
			"sku":         item.SKU,
			"name":        item.Name,
			"qty":         item.Quantity,
			"receivedQty": item.ReceivedQty,
			"price":       item.UnitCost,
			"subtotal":    item.Subtotal,
		}
	}

	etaDate := po.ETADate
	if etaDate == "" {
		etaDate = po.CreatedAt.AddDate(0, 0, 7).Format("2006-01-02")
	}

	return response.OK(c, fiber.Map{
		"id":         po.ID,
		"code":       po.PONo,
		"supplier":   po.SupplierName,
		"status":     string(po.Status),
		"amount":     po.TotalAmount,
		"totalCost":  po.TotalAmount,
		"date":       po.CreatedAt.Format("2006-01-02 15:04"),
		"etaDate":    etaDate,
		"note":       po.Note,
		"lines":      items,
		"itemsCount": len(po.Items),
	})
}

func (h *WorkspaceHandler) CreatePurchaseOrder(c *fiber.Ctx) error {
	var req struct {
		Supplier             string `json:"supplier"`
		SupplierID           uint   `json:"supplierId"`
		Note                 string `json:"note"`
		ETADate              string `json:"etaDate"`
		ExpectedDeliveryDays int    `json:"expectedDeliveryDays"`
		Items                []struct {
			SKU      string  `json:"sku"`
			Name     string  `json:"name"`
			Quantity int     `json:"quantity"`
			Qty      int     `json:"qty"`
			UnitCost float64 `json:"unitCost"`
		} `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if len(req.Items) == 0 {
		return response.BadRequest(c, "Purchase order must contain at least one item")
	}

	var supplierName string
	supplierID := req.SupplierID
	if supplierID > 0 {
		var sup domainPurchasing.Supplier
		if err := h.db.WithContext(c.Context()).First(&sup, supplierID).Error; err == nil {
			supplierName = sup.Name
		}
	} else if req.Supplier != "" {
		supplierName = req.Supplier
		var sup domainPurchasing.Supplier
		if err := h.db.WithContext(c.Context()).Where("name = ?", req.Supplier).First(&sup).Error; err == nil {
			supplierID = sup.ID
		} else {
			newSup := domainPurchasing.Supplier{
				Code:      fmt.Sprintf("SUP-%s", time.Now().Format("20060102150405")),
				Name:      req.Supplier,
				Status:    "active",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := h.db.WithContext(c.Context()).Create(&newSup).Error; err == nil {
				supplierID = newSup.ID
			}
		}
	}

	poNo := fmt.Sprintf("PO-%s-%04d", time.Now().Format("2006"), time.Now().Unix()%10000)
	var total float64
	poItems := make([]domainPurchasing.POItem, len(req.Items))
	for i, it := range req.Items {
		qty := it.Quantity
		if qty <= 0 && it.Qty > 0 {
			qty = it.Qty
		}
		if qty <= 0 {
			return response.BadRequest(c, fmt.Sprintf("Item quantity for SKU %s must be greater than 0", it.SKU))
		}
		if it.UnitCost < 0 {
			return response.BadRequest(c, fmt.Sprintf("Item unit cost for SKU %s cannot be negative", it.SKU))
		}

		sub := it.UnitCost * float64(qty)
		total += sub
		poItems[i] = domainPurchasing.POItem{
			SKU:       it.SKU,
			Name:      it.Name,
			UnitCost:  it.UnitCost,
			Quantity:  qty,
			Subtotal:  sub,
			CreatedAt: time.Now(),
		}
	}

	note := req.Note
	if req.ETADate != "" && !strings.Contains(note, "ETA:") {
		if note != "" {
			note += " "
		}
		note += fmt.Sprintf("ETA:%s", req.ETADate)
	}

	etaDate := req.ETADate
	if etaDate == "" {
		// ETA computation stays on the backend: clients may send an explicit
		// etaDate or a lead-time in days, never a pre-computed date.
		leadDays := req.ExpectedDeliveryDays
		if leadDays <= 0 {
			leadDays = 7
		}
		etaDate = time.Now().AddDate(0, 0, leadDays).Format("2006-01-02")
	}

	po := domainPurchasing.PurchaseOrder{
		PONo:         poNo,
		SupplierID:   supplierID,
		SupplierName: supplierName,
		Status:       domainPurchasing.StatusPending,
		TotalAmount:  total,
		Note:         note,
		ETADate:      etaDate,
		Items:        poItems,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.WithContext(c.Context()).Create(&po).Error; err != nil {
		return response.InternalServerError(c, "Failed to create purchase order: "+err.Error())
	}

	return response.Created(c, PurchaseRecord{
		ID:        po.ID,
		Code:      po.PONo,
		Supplier:  po.SupplierName,
		Date:      po.CreatedAt.Format("2006-01-02"),
		ETADate:   etaDate,
		TotalCost: po.TotalAmount,
		Status:    string(po.Status),
	}, "Purchase order created successfully")
}

func (h *WorkspaceHandler) UpdatePurchaseOrderStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid purchase order ID")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var po domainPurchasing.PurchaseOrder
	if err := h.db.WithContext(c.Context()).First(&po, id).Error; err != nil {
		return response.NotFound(c, "Purchase order not found")
	}

	po.Status = domainPurchasing.POStatus(strings.ToUpper(req.Status))
	po.UpdatedAt = time.Now()
	if err := h.db.WithContext(c.Context()).Save(&po).Error; err != nil {
		return response.InternalServerError(c, "Failed to update purchase order status")
	}

	return response.OK(c, po, "Purchase order status updated")
}

// QuotationRecord matching erp-web-v2 features/erp/types/records.ts
type QuotationRecord struct {
	ID         uint    `json:"id"`
	Code       string  `json:"code"`
	Customer   string  `json:"customer"`
	Date       string  `json:"date"`
	ValidUntil string  `json:"validUntil"`
	LeadSource string  `json:"leadSource"`
	Amount     float64 `json:"amount"`
	Status     string  `json:"status"`
}

func (h *WorkspaceHandler) GetQuotations(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}

	var quotations []domainQuotation.Quotation
	var total int64
	query := h.db.WithContext(c.Context()).Model(&domainQuotation.Quotation{})
	query.Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&quotations)

	records := make([]QuotationRecord, len(quotations))
	for i, q := range quotations {
		records[i] = QuotationRecord{
			ID:         q.ID,
			Code:       q.Code,
			Customer:   q.CustomerName,
			Date:       q.Date,
			ValidUntil: q.ValidUntil,
			LeadSource: q.LeadSource,
			Amount:     q.TotalAmount,
			Status:     string(q.Status),
		}
	}
	return response.List(c, records, page, limit, total)
}

// ResolveSKUs resolves a batch of SKU codes in one round trip so clients
// building quotations/POs don't issue one GET per line (N+1).
func (h *WorkspaceHandler) ResolveSKUs(c *fiber.Ctx) error {
	var req struct {
		SKUs []string `json:"skus"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if len(req.SKUs) == 0 {
		return response.BadRequest(c, "skus cannot be empty")
	}
	if len(req.SKUs) > 200 {
		return response.BadRequest(c, "skus exceeds the maximum of 200 codes per request")
	}

	normalized := make([]string, 0, len(req.SKUs))
	seen := make(map[string]bool, len(req.SKUs))
	for _, raw := range req.SKUs {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code != "" && !seen[code] {
			seen[code] = true
			normalized = append(normalized, code)
		}
	}
	if len(normalized) == 0 {
		return response.BadRequest(c, "skus cannot be empty")
	}

	var skus []domainSKU.SKU
	if err := h.db.WithContext(c.Context()).
		Where("UPPER(TRIM(sku)) IN ?", normalized).
		Find(&skus).Error; err != nil {
		return response.InternalServerError(c, "Failed to resolve SKUs")
	}

	found := make(map[string]domainSKU.SKU, len(skus))
	for _, s := range skus {
		found[strings.ToUpper(strings.TrimSpace(s.SKU))] = s
	}

	type ResolvedSKU struct {
		Sku    string `json:"sku"`
		Id     uint   `json:"id"`
		Name   string `json:"name"`
		Found  bool   `json:"found"`
	}
	records := make([]ResolvedSKU, 0, len(normalized))
	for _, code := range normalized {
		s, ok := found[code]
		if !ok {
			records = append(records, ResolvedSKU{Sku: code, Found: false})
			continue
		}
		records = append(records, ResolvedSKU{Sku: s.SKU, Id: s.ID, Name: s.Name, Found: true})
	}

	return response.OK(c, records)
}

func (h *WorkspaceHandler) CreateQuotation(c *fiber.Ctx) error {
	var req struct {
		Customer   string `json:"customer"`
		Date       string `json:"date"`
		ValidUntil string `json:"validUntil"`
		Status     string `json:"status"`
		LeadSource string `json:"leadSource"`
		Note       string `json:"note"`
		Lines      []struct {
			ProductID uint    `json:"productId"`
			SKU       string  `json:"sku"`
			Name      string  `json:"name"`
			Price     float64 `json:"price"`
			Qty       int     `json:"qty"`
		} `json:"lines"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Customer == "" {
		return response.BadRequest(c, "Customer name is required")
	}
	if len(req.Lines) == 0 {
		return response.BadRequest(c, "At least one item line is required")
	}

	code := fmt.Sprintf("QT-%s-%04d", time.Now().Format("2006"), time.Now().Unix()%10000)
	var total float64
	lines := make([]domainQuotation.QuotationLine, len(req.Lines))
	for i, l := range req.Lines {
		sub := l.Price * float64(l.Qty)
		total += sub
		lines[i] = domainQuotation.QuotationLine{
			ProductID: l.ProductID,
			SKU:       l.SKU,
			Name:      l.Name,
			Price:     l.Price,
			Quantity:  l.Qty,
			Subtotal:  sub,
			CreatedAt: time.Now(),
		}
	}

	status := domainQuotation.StatusDraft
	if req.Status != "" {
		status = domainQuotation.Status(req.Status)
	}

	dateStr := req.Date
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	validUntilStr := req.ValidUntil
	if validUntilStr == "" {
		validUntilStr = time.Now().AddDate(0, 0, 15).Format("2006-01-02")
	}

	q := domainQuotation.Quotation{
		Code:         code,
		CustomerName: req.Customer,
		Date:         dateStr,
		ValidUntil:   validUntilStr,
		LeadSource:   req.LeadSource,
		Status:       status,
		TotalAmount:  total,
		Note:         req.Note,
		Lines:        lines,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.WithContext(c.Context()).Create(&q).Error; err != nil {
		return response.InternalServerError(c, "Failed to create quotation: "+err.Error())
	}

	rec := QuotationRecord{
		ID:         q.ID,
		Code:       q.Code,
		Customer:   q.CustomerName,
		Date:       q.Date,
		ValidUntil: q.ValidUntil,
		LeadSource: q.LeadSource,
		Amount:     q.TotalAmount,
		Status:     string(q.Status),
	}
	return response.Created(c, rec, "Quotation created successfully")
}

func (h *WorkspaceHandler) GetQuotationByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	var q domainQuotation.Quotation
	if err := h.db.WithContext(c.Context()).Preload("Lines").First(&q, id).Error; err != nil {
		return response.NotFound(c, "Quotation not found")
	}

	lines := make([]fiber.Map, len(q.Lines))
	for i, l := range q.Lines {
		lines[i] = fiber.Map{
			"id":        l.ID,
			"productId": l.ProductID,
			"sku":       l.SKU,
			"name":      l.Name,
			"price":     l.Price,
			"qty":       l.Quantity,
			"subtotal":  l.Subtotal,
		}
	}

	return response.OK(c, fiber.Map{
		"id":         q.ID,
		"code":       q.Code,
		"customer":   q.CustomerName,
		"date":       q.Date,
		"validUntil": q.ValidUntil,
		"amount":     q.TotalAmount,
		"status":     string(q.Status),
		"leadSource": q.LeadSource,
		"note":       q.Note,
		"lines":      lines,
		"itemsCount": len(q.Lines),
	})
}

func (h *WorkspaceHandler) UpdateQuotationStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	var q domainQuotation.Quotation
	if err := h.db.WithContext(c.Context()).First(&q, id).Error; err != nil {
		return response.NotFound(c, "Quotation not found")
	}

	q.Status = domainQuotation.Status(req.Status)
	q.UpdatedAt = time.Now()
	if err := h.db.WithContext(c.Context()).Save(&q).Error; err != nil {
		return response.InternalServerError(c, "Failed to update quotation status")
	}

	return response.OK(c, q, "Quotation status updated")
}

func (h *WorkspaceHandler) ConvertQuotationToSO(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid quotation ID")
	}

	var q domainQuotation.Quotation
	if err := h.db.WithContext(c.Context()).Preload("Lines").First(&q, id).Error; err != nil {
		return response.NotFound(c, "Quotation not found")
	}

	if q.Status == domainQuotation.StatusConverted {
		return response.BadRequest(c, "Quotation is already converted")
	}

	var orderID uint
	var orderNo string

	err = h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		orderNo = fmt.Sprintf("SO-%s-%04d", time.Now().Format("2006"), time.Now().Unix()%10000)
		ordItems := make([]domainOrder.OrderItem, len(q.Lines))
		for i, l := range q.Lines {
			ordItems[i] = domainOrder.OrderItem{
				SKU:       l.SKU,
				Name:      l.Name,
				Price:     l.Price,
				Quantity:  l.Quantity,
				Subtotal:  l.Subtotal,
				CreatedAt: time.Now(),
			}
		}

		channel := "direct"
		if q.LeadSource != "" {
			channel = strings.ToLower(q.LeadSource)
		}

		ord := domainOrder.Order{
			OrderNo:      orderNo,
			CustomerName: q.CustomerName,
			Channel:      channel,
			Status:       domainOrder.StatusPending,
			TotalAmount:  q.TotalAmount,
			Note:         fmt.Sprintf("Converted from Quotation %s", q.Code),
			Items:        ordItems,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := tx.Create(&ord).Error; err != nil {
			return err
		}
		orderID = ord.ID

		q.Status = domainQuotation.StatusConverted
		q.UpdatedAt = time.Now()
		return tx.Save(&q).Error
	})

	if err != nil {
		return response.InternalServerError(c, "Failed to convert quotation: "+err.Error())
	}

	return response.OK(c, fiber.Map{
		"quotationId": q.ID,
		"orderId":     orderID,
		"orderNo":     orderNo,
	}, "Quotation converted to Sales Order successfully")
}

// ReceiptRecord matching erp-web-v2 features/erp/types/records.ts
type ReceiptRecord struct {
	ID          uint   `json:"id"`
	Code        string `json:"code"`
	PORef       string `json:"poRef"`
	ReceiveDate string `json:"receiveDate"`
	Note        string `json:"note"`
}

func (h *WorkspaceHandler) GetGoodsReceives(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}

	var grs []domainPurchasing.GoodsReceive
	var total int64
	query := h.db.WithContext(c.Context()).Model(&domainPurchasing.GoodsReceive{})
	if search := strings.TrimSpace(c.Query("search", "")); search != "" {
		s := "%" + search + "%"
		query = query.Where("code ILIKE ? OR po_ref ILIKE ? OR supplier_name ILIKE ?", s, s, s)
	}
	query.Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&grs)

	records := make([]ReceiptRecord, len(grs))
	for i, gr := range grs {
		receiveDate := gr.ReceiveDate
		if receiveDate == "" {
			receiveDate = gr.CreatedAt.Format("2006-01-02")
		}
		records[i] = ReceiptRecord{
			ID:          gr.ID,
			Code:        gr.Code,
			PORef:       gr.PORef,
			ReceiveDate: receiveDate,
			Note:        gr.Note,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) GetGoodsReceiveByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid goods receive ID")
	}

	var gr domainPurchasing.GoodsReceive
	if err := h.db.WithContext(c.Context()).Preload("Items").First(&gr, id).Error; err != nil {
		return response.NotFound(c, "Goods receive not found")
	}

	items := make([]fiber.Map, len(gr.Items))
	for i, item := range gr.Items {
		items[i] = fiber.Map{
			"id":          item.ID,
			"sku":         item.SKU,
			"name":        item.Name,
			"qty":         item.Quantity,
			"receivedQty": item.Quantity,
			"supplierLot": item.SupplierLot,
			"expiryDate":  item.ExpiryDate,
			"qcStatus":    item.QCStatus,
		}
	}

	receiveDate := gr.ReceiveDate
	if receiveDate == "" {
		receiveDate = gr.CreatedAt.Format("2006-01-02")
	}

	return response.OK(c, fiber.Map{
		"id":          gr.ID,
		"code":        gr.Code,
		"poRef":       gr.PORef,
		"receiveDate": receiveDate,
		"supplier":    gr.SupplierName,
		"note":        gr.Note,
		"lines":       items,
		"itemsCount":  len(gr.Items),
	})
}

func (h *WorkspaceHandler) CreateGoodsReceive(c *fiber.Ctx) error {
	var req struct {
		PORef       string `json:"poRef"`
		ReceiveDate string `json:"receiveDate"`
		Note        string `json:"note"`
		Items       []struct {
			SKU         string `json:"sku"`
			QtyReceived int    `json:"qtyReceived"`
			Quantity    int    `json:"quantity"`
			ExpiryDate  string `json:"expiryDate"`
			SupplierLot string `json:"supplierLot"`
			QCStatus    string `json:"qcStatus"`
		} `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	receiveDate := req.ReceiveDate
	if receiveDate == "" {
		receiveDate = time.Now().Format("2006-01-02")
	}

	hasPO := strings.TrimSpace(req.PORef) != ""
	var po domainPurchasing.PurchaseOrder
	if hasPO {
		if err := h.db.WithContext(c.Context()).Where("po_no = ? OR id::text = ?", req.PORef, req.PORef).Preload("Items").First(&po).Error; err != nil {
			return response.NotFound(c, "Purchase order not found")
		}

		if po.Status == domainPurchasing.StatusReceived {
			return response.BadRequest(c, fmt.Sprintf("Purchase order %s has already been received in full", po.PONo))
		}
		if po.Status == domainPurchasing.StatusCancelled {
			return response.BadRequest(c, fmt.Sprintf("Purchase order %s is cancelled", po.PONo))
		}
	}

	type receiveItem struct {
		SKU         string
		Quantity    int
		SupplierLot string
		ExpiryDate  string
		QCStatus    string
	}
	var itemsToReceive []receiveItem

	if len(req.Items) > 0 {
		for _, it := range req.Items {
			qty := it.QtyReceived
			if qty <= 0 && it.Quantity > 0 {
				qty = it.Quantity
			}
			if it.SKU != "" && qty > 0 {
				qc := it.QCStatus
				if qc == "" {
					qc = "Accepted"
				}
				itemsToReceive = append(itemsToReceive, receiveItem{
					SKU:         strings.ToUpper(strings.TrimSpace(it.SKU)),
					Quantity:    qty,
					SupplierLot: it.SupplierLot,
					ExpiryDate:  it.ExpiryDate,
					QCStatus:    qc,
				})
			}
		}
	} else if hasPO {
		for _, it := range po.Items {
			remaining := it.Quantity - it.ReceivedQty
			if remaining > 0 {
				itemsToReceive = append(itemsToReceive, receiveItem{
					SKU:      strings.ToUpper(strings.TrimSpace(it.SKU)),
					Quantity: remaining,
					QCStatus: "Accepted",
				})
			}
		}
	}

	if len(itemsToReceive) == 0 {
		return response.BadRequest(c, "No remaining items to receive")
	}

	var createdGR domainPurchasing.GoodsReceive

	err := h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		refID := req.PORef
		if refID == "" {
			refID = fmt.Sprintf("GR-%d", time.Now().Unix())
		}

		var poIDPtr *uint
		supplierName := ""

		if hasPO {
			// Lock the PO row for update to prevent concurrent double receive
			var lockedPO domainPurchasing.PurchaseOrder
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").First(&lockedPO, po.ID).Error; err != nil {
				return err
			}

			if lockedPO.Status == domainPurchasing.StatusReceived {
				return errors.New("Purchase order has already been received")
			}

			poIDPtr = &lockedPO.ID
			supplierName = lockedPO.SupplierName

			// FULL-14: map PO lines to a slice so duplicate SKUs on separate
			// lines are honored instead of collapsing to the last line.
			poLineIndex := make(map[string][]*domainPurchasing.POItem)
			for i := range lockedPO.Items {
				skuKey := strings.ToUpper(strings.TrimSpace(lockedPO.Items[i].SKU))
				poLineIndex[skuKey] = append(poLineIndex[skuKey], &lockedPO.Items[i])
			}

			// Validate all receive items: must exist in PO and must not exceed
			// remaining quantity, aggregating across duplicate PO lines.
			// FULL-10: QC-rejected units are also validated against the PO
			// (they physically arrive against this PO) but are not counted as
			// received for fulfillment purposes — see receivedAccepted below.
			receivedAgg := make(map[string]int)
			for _, recIt := range itemsToReceive {
				lines, exists := poLineIndex[recIt.SKU]
				if !exists || len(lines) == 0 {
					return fmt.Errorf("SKU %s does not belong to purchase order %s", recIt.SKU, lockedPO.PONo)
				}
				remaining := 0
				for _, poIt := range lines {
					remaining += poIt.Quantity - poIt.ReceivedQty
				}
				receivedAgg[recIt.SKU] += recIt.Quantity
				if receivedAgg[recIt.SKU] > remaining {
					return fmt.Errorf("quantity to receive for SKU %s (%d) exceeds remaining PO quantity (%d)", recIt.SKU, receivedAgg[recIt.SKU], remaining)
				}
			}

			// FULL-10: only QC-accepted units advance PO fulfillment; rejected
			// units keep the line open so the supplier can re-deliver.
			receivedAccepted := make(map[string]int)
			for _, recIt := range itemsToReceive {
				qcUpper := strings.ToUpper(strings.TrimSpace(recIt.QCStatus))
				if qcUpper == "" || qcUpper == "ACCEPTED" || qcUpper == "PASS" || qcUpper == "PASSED" {
					receivedAccepted[recIt.SKU] += recIt.Quantity
				}
			}

			// FULL-14: distribute received quantities across the PO's duplicate
			// lines in order, filling each line's remaining amount first.
			// Only QC-accepted quantities advance fulfillment (FULL-10).
			for skuKey, qtyAdded := range receivedAccepted {
				left := qtyAdded
				for _, poIt := range poLineIndex[skuKey] {
					if left <= 0 {
						break
					}
					lineRemaining := poIt.Quantity - poIt.ReceivedQty
					if lineRemaining <= 0 {
						continue
					}
					take := lineRemaining
					if left < take {
						take = left
					}
					poIt.ReceivedQty += take
					left -= take
					if err := tx.Model(&domainPurchasing.POItem{}).Where("id = ?", poIt.ID).
						Update("received_qty", poIt.ReceivedQty).Error; err != nil {
						return err
					}
				}
			}

			// Check if all items are fully received
			allDone := true
			for i := range lockedPO.Items {
				if lockedPO.Items[i].ReceivedQty < lockedPO.Items[i].Quantity {
					allDone = false
					break
				}
			}

			if allDone {
				lockedPO.Status = domainPurchasing.StatusReceived
			}
			lockedPO.UpdatedAt = time.Now()
			if req.Note != "" {
				lockedPO.Note = req.Note
			}
			if err := tx.Save(&lockedPO).Error; err != nil {
				return err
			}
			po = lockedPO
		}

		grCode := fmt.Sprintf("GR-%s", time.Now().Format("20060102150405"))
		createdGR = domainPurchasing.GoodsReceive{
			Code:         grCode,
			POID:         poIDPtr,
			PORef:        req.PORef,
			SupplierName: supplierName,
			ReceiveDate:  receiveDate,
			Note:         req.Note,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := tx.Create(&createdGR).Error; err != nil {
			return err
		}

		for _, it := range itemsToReceive {
			var s domainSKU.SKU
			if err := tx.Where("UPPER(TRIM(sku)) = ?", it.SKU).First(&s).Error; err != nil {
				return fmt.Errorf("SKU %s not found in system", it.SKU)
			}

			grItem := domainPurchasing.GoodsReceiveItem{
				GRID:        createdGR.ID,
				SKU:         s.SKU,
				Name:        s.Name,
				Quantity:    it.Quantity,
				SupplierLot: it.SupplierLot,
				ExpiryDate:  it.ExpiryDate,
				QCStatus:    it.QCStatus,
				CreatedAt:   time.Now(),
			}
			if err := tx.Create(&grItem).Error; err != nil {
				return err
			}

			// FULL-10: only QC-Accepted units become sellable stock. Rejected
			// units are recorded on the GR for traceability but never added to
			// Quantity/AvailableQty.
			qcUpper := strings.ToUpper(strings.TrimSpace(it.QCStatus))
			qcAccepted := qcUpper == "" || qcUpper == "ACCEPTED" || qcUpper == "PASS" || qcUpper == "PASSED"

			var stk domainStock.Stock
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sku_id = ? AND warehouse_id = ?", s.ID, 1).First(&stk).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					if qcAccepted {
						stk = domainStock.Stock{
							SKUID:        s.ID,
							SKUCode:      s.SKU,
							WarehouseID:  1,
							Quantity:     it.Quantity,
							AvailableQty: it.Quantity,
							UpdatedAt:    time.Now(),
						}
						if err := tx.Create(&stk).Error; err != nil {
							return err
						}
					}
				} else {
					return err
				}
			} else if qcAccepted {
				stk.Quantity += it.Quantity
				stk.AvailableQty += it.Quantity
				stk.UpdatedAt = time.Now()
				if err := tx.Save(&stk).Error; err != nil {
					return err
				}
			}

			note := req.Note
			if it.SupplierLot != "" || it.ExpiryDate != "" {
				note = fmt.Sprintf("%s [Lot: %s, Exp: %s]", note, it.SupplierLot, it.ExpiryDate)
			}
			if !qcAccepted {
				if note != "" {
					note += " "
				}
				note += fmt.Sprintf("[QC %s — not added to sellable stock]", it.QCStatus)
			}

			// Rejected units still generate a movement trail, but as a
			// QC_REJECT type with zero stock effect.
			movementType := domainStock.MovementIn
			movementQty := it.Quantity
			if !qcAccepted {
				movementType = domainStock.MovementType("QC_REJECT")
				movementQty = 0
			}

			if err := tx.Create(&domainStock.StockMovement{
				SKUID:         s.ID,
				SKUCode:       s.SKU,
				WarehouseID:   1,
				Type:          movementType,
				Quantity:      movementQty,
				BeforeQty:     stk.Quantity,
				AfterQty:      stk.Quantity,
				ReferenceType: "GOODS_RECEIVE",
				ReferenceID:   createdGR.Code,
				Note:          strings.TrimSpace(note),
				CreatedAt:     time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return response.BadRequest(c, "Failed to receive goods: "+err.Error())
	}

	return response.Created(c, ReceiptRecord{
		ID:          createdGR.ID,
		Code:        createdGR.Code,
		PORef:       createdGR.PORef,
		ReceiveDate: createdGR.ReceiveDate,
		Note:        createdGR.Note,
	}, "Goods received successfully")
}

// IssueRecord matching erp-web-v2 features/erp/types/records.ts
type IssueRecord struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	SKU      string `json:"sku"`
	SKUName  string `json:"skuName"`
	Qty      int    `json:"qty"`
	Reason   string `json:"reason"`
	Date     string `json:"date"`
	Channel  string `json:"channel"`
	OrderRef string `json:"orderRef"`
}

// formatMovementChannel formats raw reference type or channel to user-friendly "Manual", "Shopee", "TikTok"
func formatMovementChannel(channel, orderRef string) string {
	u := strings.ToUpper(strings.TrimSpace(channel))
	if strings.Contains(u, "TIKTOK") {
		return "TikTok"
	}
	if strings.Contains(u, "SHOPEE") {
		return "Shopee"
	}
	if strings.Contains(u, "MANUAL") {
		return "Manual"
	}
	if strings.Contains(u, "ORDER") {
		ref := strings.ToUpper(strings.TrimSpace(orderRef))
		if strings.HasPrefix(ref, "TT") || strings.Contains(ref, "TIKTOK") {
			return "TikTok"
		}
		if strings.HasPrefix(ref, "SP") || strings.Contains(ref, "SHOPEE") {
			return "Shopee"
		}
		return "Manual"
	}
	if channel == "" {
		return "Manual"
	}
	return channel
}

func (h *WorkspaceHandler) GetGoodsIssues(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}

	// Fetch OUT stock movements for goods issue history
	var movements []domainStock.StockMovement
	var total int64
	query := h.db.WithContext(c.Context()).Model(&domainStock.StockMovement{}).
		Where("type = ?", domainStock.MovementOut)
	if search := strings.TrimSpace(c.Query("search", "")); search != "" {
		s := "%" + search + "%"
		query = query.Where("sku_code ILIKE ? OR note ILIKE ? OR reference_id ILIKE ?", s, s, s)
	}
	query.Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Order("id DESC").Find(&movements)

	records := make([]IssueRecord, len(movements))
	for i, m := range movements {
		records[i] = IssueRecord{
			ID:       m.ID,
			Code:     fmt.Sprintf("GI-%04d", m.ID),
			SKU:      m.SKUCode,
			SKUName:  m.SKUCode,
			Qty:      m.Quantity,
			Reason:   m.Note,
			Date:     m.CreatedAt.Format("2006-01-02"),
			Channel:  formatMovementChannel(m.ReferenceType, m.ReferenceID),
			OrderRef: m.ReferenceID,
		}
	}

	return response.List(c, records, page, limit, total)
}

func (h *WorkspaceHandler) GetGoodsIssueByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid goods issue ID")
	}

	var m domainStock.StockMovement
	if err := h.db.WithContext(c.Context()).First(&m, id).Error; err != nil {
		return response.NotFound(c, "Goods issue not found")
	}

	return response.OK(c, fiber.Map{
		"id":       m.ID,
		"code":     fmt.Sprintf("GI-%04d", m.ID),
		"sku":      m.SKUCode,
		"skuName":  m.SKUCode,
		"qty":      m.Quantity,
		"reason":   m.Note,
		"date":     m.CreatedAt.Format("2006-01-02 15:04"),
		"channel":  formatMovementChannel(m.ReferenceType, m.ReferenceID),
		"orderRef": m.ReferenceID,
	})
}

func (h *WorkspaceHandler) CreateGoodsIssue(c *fiber.Ctx) error {
	var req struct {
		SKU      string `json:"sku"`
		Qty      int    `json:"qty"`
		Reason   string `json:"reason"`
		Channel  string `json:"channel"`
		OrderRef string `json:"orderRef"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.SKU == "" || req.Qty <= 0 {
		return response.BadRequest(c, "SKU and positive quantity are required")
	}

	var s domainSKU.SKU
	if err := h.db.WithContext(c.Context()).Where("UPPER(TRIM(sku)) = ?", strings.ToUpper(strings.TrimSpace(req.SKU))).First(&s).Error; err != nil {
		return response.NotFound(c, "SKU not found")
	}

	refType := req.Channel
	if refType == "" {
		refType = "MANUAL_ISSUE"
	}

	var movement domainStock.StockMovement
	err := h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		var stk domainStock.Stock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sku_id = ? AND warehouse_id = ?", s.ID, 1).First(&stk).Error; err != nil {
			return errors.New("Stock not found for SKU")
		}

		if stk.Quantity < req.Qty {
			return fmt.Errorf("insufficient total stock: current %d, required %d", stk.Quantity, req.Qty)
		}

		if stk.AvailableQty < req.Qty {
			return fmt.Errorf("insufficient available stock (allocated for reservations): available %d, required %d", stk.AvailableQty, req.Qty)
		}

		before := stk.Quantity
		stk.Quantity -= req.Qty
		stk.AvailableQty -= req.Qty
		stk.UpdatedAt = time.Now()
		if err := tx.Save(&stk).Error; err != nil {
			return err
		}

		movement = domainStock.StockMovement{
			SKUID:         s.ID,
			SKUCode:       s.SKU,
			WarehouseID:   1,
			Type:          domainStock.MovementOut,
			Quantity:      req.Qty,
			BeforeQty:     before,
			AfterQty:      stk.Quantity,
			ReferenceType: refType,
			ReferenceID:   req.OrderRef,
			Note:          req.Reason,
			CreatedAt:     time.Now(),
		}
		return tx.Create(&movement).Error
	})

	if err != nil {
		return response.BadRequest(c, "Failed to create goods issue: "+err.Error())
	}

	return response.Created(c, IssueRecord{
		ID:       movement.ID,
		Code:     fmt.Sprintf("GI-%04d", movement.ID),
		SKU:      movement.SKUCode,
		SKUName:  movement.SKUCode,
		Qty:      movement.Quantity,
		Reason:   movement.Note,
		Date:     movement.CreatedAt.Format("2006-01-02"),
		Channel:  formatMovementChannel(movement.ReferenceType, movement.ReferenceID),
		OrderRef: movement.ReferenceID,
	}, "Goods issued successfully")
}

func (h *WorkspaceHandler) AdjustStock(c *fiber.Ctx) error {
	var req struct {
		Note  string `json:"note"`
		Items []struct {
			SKU       string `json:"sku"`
			ActualQty int    `json:"actualQty"`
		} `json:"items"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if len(req.Items) == 0 {
		return response.BadRequest(c, "Items cannot be empty")
	}

	err := h.db.WithContext(c.Context()).Transaction(func(tx *gorm.DB) error {
		for _, it := range req.Items {
			if it.ActualQty < 0 {
				return fmt.Errorf("actual quantity for SKU %s cannot be negative (%d)", it.SKU, it.ActualQty)
			}

			var s domainSKU.SKU
			if err := tx.Where("UPPER(TRIM(sku)) = ?", strings.ToUpper(strings.TrimSpace(it.SKU))).First(&s).Error; err != nil {
				return fmt.Errorf("SKU %s not found", it.SKU)
			}

			var stk domainStock.Stock
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("sku_id = ? AND warehouse_id = ?", s.ID, 1).First(&stk).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// Initialize stock record if not yet created
					stk = domainStock.Stock{
						SKUID:        s.ID,
						SKUCode:      s.SKU,
						WarehouseID:  1,
						Quantity:     0,
						ReservedQty:  0,
						AvailableQty: 0,
						UpdatedAt:    time.Now(),
					}
					if err := tx.Create(&stk).Error; err != nil {
						return err
					}
				} else {
					return fmt.Errorf("failed to query stock for SKU %s: %w", it.SKU, err)
				}
			}

			if it.ActualQty < stk.ReservedQty {
				return fmt.Errorf("cannot adjust quantity to %d for SKU %s because it is lower than reserved quantity %d", it.ActualQty, it.SKU, stk.ReservedQty)
			}

			delta := it.ActualQty - stk.Quantity
			stk.Quantity = it.ActualQty
			stk.AvailableQty = it.ActualQty - stk.ReservedQty
			stk.UpdatedAt = time.Now()
			if err := tx.Save(&stk).Error; err != nil {
				return err
			}

			movement := domainStock.StockMovement{
				SKUID:         s.ID,
				SKUCode:       s.SKU,
				WarehouseID:   1,
				Type:          domainStock.MovementAdjust,
				Quantity:      delta,
				BeforeQty:     stk.Quantity - delta,
				AfterQty:      stk.Quantity,
				ReferenceType: "MANUAL_ADJUSTMENT",
				Note:          req.Note,
				CreatedAt:     time.Now(),
			}
			if err := tx.Create(&movement).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return response.BadRequest(c, "Failed to adjust stock: "+err.Error())
	}

	return response.OK(c, nil, "Stock adjusted successfully")
}

// BundleComponentRecord for frontend compatibility (erp-web-v2 and erp-web)
type BundleComponentRecord struct {
	ID           uint    `json:"id"`
	BundleSKU    string  `json:"bundleSku"`
	ComponentSKU string  `json:"componentSku"`
	Qty          float64 `json:"qty"`
	Note         string  `json:"note"`
}

func (h *WorkspaceHandler) GetBundleComponents(c *fiber.Ctx) error {
	sku := c.Params("sku")
	var items []domainBundle.BundleItem
	query := h.db.WithContext(c.Context()).Model(&domainBundle.BundleItem{})
	if sku != "" {
		query = query.Where("bundle_sku = ?", strings.ToUpper(strings.TrimSpace(sku)))
	}
	if err := query.Find(&items).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch bundle components: "+err.Error())
	}

	records := make([]BundleComponentRecord, len(items))
	for i, item := range items {
		records[i] = BundleComponentRecord{
			ID:           item.ID,
			BundleSKU:    item.BundleSKU,
			ComponentSKU: item.ComponentSKU,
			Qty:          float64(item.Quantity),
			Note:         item.Note,
		}
	}

	return response.OK(c, records)
}
