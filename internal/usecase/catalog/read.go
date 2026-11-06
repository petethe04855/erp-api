package catalog

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainBundle "chawy-erp-api/internal/domain/bundle"
	domainCatalog "chawy-erp-api/internal/domain/catalog"
	domainFormula "chawy-erp-api/internal/domain/formula"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	appErrors "chawy-erp-api/pkg/errors"
)

// List reproduces the workspace GetProducts aggregation exactly: stock sums,
// used quantities (movement OUT), TikTok reserved via SKU mappings + pending
// orders, bundle component availability and receipt stats are batched once for
// the page, and each row is shaped for erp-web-v2 records.
func (u *usecase) List(ctx context.Context, q domainCatalog.ProductQuery) ([]ProductRecord, int64, error) {
	total, err := u.repo.Count(ctx, q)
	if err != nil {
		return nil, 0, dbErr500("Failed to count products: " + err.Error())
	}

	skus, err := u.repo.Search(ctx, q)
	if err != nil {
		return nil, total, dbErr500("Failed to fetch products: " + err.Error())
	}

	if len(skus) == 0 {
		out := []ProductRecord{}
		return out, total, nil
	}

	skuIDs := make([]uint, len(skus))
	skuCodes := make([]string, len(skus))
	for i, s := range skus {
		skuIDs[i] = s.ID
		skuCodes[i] = strings.ToUpper(strings.TrimSpace(s.SKU))
	}

	// 1. Batch fetch stocks aggregated across all warehouses for all retrieved SKUs
	stockSums, err := u.repo.SumStocks(ctx, skuIDs)
	if err != nil {
		return nil, total, dbErr500("Failed to fetch stock sums: " + err.Error())
	}
	stockMap := make(map[uint]domainCatalog.StockSumRow, len(stockSums))
	for _, ss := range stockSums {
		stockMap[ss.SKUID] = ss
	}

	// 1.1 Batch fetch total used quantity (movement type OUT) for retrieved SKUs
	usedSums, _ := u.repo.SumUsed(ctx, skuIDs, skuCodes)
	usedMap := make(map[uint]int)
	usedCodeMap := make(map[string]int)
	for _, us := range usedSums {
		if us.SKUID > 0 {
			usedMap[us.SKUID] += us.Qty
		}
		if us.SKUCode != "" {
			usedCodeMap[us.SKUCode] += us.Qty
		}
	}

	// 2. Batch fetch TikTok SKU mappings for candidate SKU sets
	mappings, err := u.repo.TikTokMappings(ctx, skuCodes)
	if err != nil {
		return nil, total, dbErr500("Failed to fetch TikTok mappings: " + err.Error())
	}

	// Build map of erp_sku -> candidate TikTok SKUs
	skuToCandidates := make(map[string]map[string]bool, len(skus))
	var allCandidates []string
	seenCandidate := make(map[string]bool)
	for _, sCode := range skuCodes {
		if sCode == "" {
			continue
		}
		if _, ok := skuToCandidates[sCode]; !ok {
			skuToCandidates[sCode] = make(map[string]bool)
		}
		skuToCandidates[sCode][sCode] = true
		if !seenCandidate[sCode] {
			seenCandidate[sCode] = true
			allCandidates = append(allCandidates, sCode)
		}
	}
	for _, m := range mappings {
		erpUpper := strings.ToUpper(strings.TrimSpace(m.LocalSKU))
		ttUpper := strings.ToUpper(strings.TrimSpace(m.TikTokSKU))
		if erpUpper != "" && ttUpper != "" {
			if _, ok := skuToCandidates[erpUpper]; !ok {
				skuToCandidates[erpUpper] = make(map[string]bool)
			}
			skuToCandidates[erpUpper][ttUpper] = true
			if !seenCandidate[ttUpper] {
				seenCandidate[ttUpper] = true
				allCandidates = append(allCandidates, ttUpper)
			}
		}
	}

	// 3. Batch fetch TikTok pending reserved quantities grouped by SKU
	tiktokPendingMap := make(map[string]int64)
	if len(allCandidates) > 0 {
		pendingItems, err := u.repo.TikTokPending(ctx, allCandidates)
		if err != nil {
			return nil, total, dbErr500("Failed to calculate TikTok pending reserves: " + err.Error())
		}
		for _, pi := range pendingItems {
			tiktokPendingMap[pi.SKU] = pi.Qty
		}
	}

	// 4. Batch fetch bundle items and accessories for candidate SKUs
	var bundleSKUs []string
	var nonBundleSKUs []string
	for _, s := range skus {
		if s.IsBundle {
			bundleSKUs = append(bundleSKUs, s.SKU)
		} else {
			nonBundleSKUs = append(nonBundleSKUs, s.SKU)
		}
	}

	bundleItemsMap := make(map[string][]domainBundle.BundleItem)
	if len(bundleSKUs) > 0 {
		bItems, _ := u.repo.BundleItems(ctx, bundleSKUs)
		for _, bi := range bItems {
			bundleItemsMap[bi.BundleSKU] = append(bundleItemsMap[bi.BundleSKU], bi)
		}
	}

	// Fetch components stock availability for virtual bundles
	var compSKUList []string
	seenComp := make(map[string]bool)
	for _, items := range bundleItemsMap {
		for _, bi := range items {
			cUpper := strings.ToUpper(strings.TrimSpace(bi.ComponentSKU))
			if cUpper != "" && !seenComp[cUpper] {
				seenComp[cUpper] = true
				compSKUList = append(compSKUList, cUpper)
			}
		}
	}

	compAvailableMap := make(map[string]int)
	if len(compSKUList) > 0 {
		compStocks, _ := u.repo.ComponentAvailability(ctx, compSKUList)
		for _, cs := range compStocks {
			compAvailableMap[cs.SKUCode] = cs.AvailableQty
		}
	}

	// Batch fetch accessories for non-bundle SKUs
	accessoriesMap := make(map[string][]SKUAccessoryRecord)
	if len(nonBundleSKUs) > 0 {
		accItems, _ := u.repo.AccessoriesForAll(ctx, nonBundleSKUs)
		for _, ai := range accItems {
			accessoriesMap[ai.SKU] = append(accessoriesMap[ai.SKU], SKUAccessoryRecord{
				ID:           ai.ID,
				SKU:          ai.SKU,
				AccessorySKU: ai.AccessorySKU,
				Quantity:     ai.Quantity,
				Note:         ai.Note,
			})
		}
	}

	// Batch fetch receipt stats for candidate SKUs
	var receiptStatsMap map[uint]domainSKU.SKUBatchReceiptStat
	if u.stats != nil {
		receiptStatsMap, _ = u.stats.GetReceiptStatsBatch(ctx, skuIDs)
	}

	records := make([]ProductRecord, len(skus))
	for i, s := range skus {
		stk := stockMap[s.ID]

		pType := "Finished Product"
		if s.Category != "" {
			pType = s.Category
		}

		sCode := strings.ToUpper(strings.TrimSpace(s.SKU))
		var tiktokReserved int64
		if cands, ok := skuToCandidates[sCode]; ok {
			for cand := range cands {
				tiktokReserved += tiktokPendingMap[cand]
			}
		}

		totalReserved := stk.ReservedQty + int(tiktokReserved)
		available := stk.Quantity - totalReserved
		if available < 0 {
			available = 0
		}

		var bundleAvail *int
		if s.IsBundle {
			items := bundleItemsMap[s.SKU]
			if len(items) == 0 {
				zero := 0
				bundleAvail = &zero
			} else {
				minSets := -1
				for _, bi := range items {
					reqQty := bi.Quantity
					if reqQty <= 0 {
						reqQty = 1
					}
					cStock := compAvailableMap[strings.ToUpper(strings.TrimSpace(bi.ComponentSKU))]
					sets := cStock / reqQty
					if minSets == -1 || sets < minSets {
						minSets = sets
					}
				}
				if minSets < 0 {
					minSets = 0
				}
				bundleAvail = &minSets
			}
		}

		usedTotal := usedMap[s.ID]
		if alt, ok := usedCodeMap[sCode]; ok && alt > usedTotal {
			usedTotal = alt
		}

		rcStat := receiptStatsMap[s.ID]
		var lastRecDateStr *string
		if rcStat.LastReceivedAt != nil {
			formatted := rcStat.LastReceivedAt.Format(time.RFC3339)
			lastRecDateStr = &formatted
		}

		records[i] = ProductRecord{
			ID:              s.ID,
			SKU:             s.SKU,
			Name:            s.Name,
			Type:            pType,
			Barcode:         s.Barcode,
			BaseUnit:        "ชิ้น",
			RetailPrice:     s.Price,
			WholesalePrice:  s.Price,
			Cost:            s.CostPrice,
			Stock:           stk.Quantity,
			ReservedQty:     totalReserved,
			UsedQty:         usedTotal,
			Reorder:         s.ReorderPoint,
			IsBundle:        s.IsBundle,
			IsActive:        s.Status == "active",
			Available:       available,
			BundleAvailable: bundleAvail,
			Image:           s.Image,
			Accessories:     accessoriesMap[s.SKU],
			CreatedAt:       s.CreatedAt.Format(time.RFC3339),
			LastReceivedAt:  lastRecDateStr,
			ReceiptCount:    rcStat.ReceiptCount,
		}
	}

	return records, total, nil
}

// GetByCode reproduces the single-product detail view of the workspace
// GetProductByCode endpoint. Database failures of the auxiliary queries keep
// the original swallow-the-error behaviour; only the missing product matters.
func (u *usecase) GetByCode(ctx context.Context, code string) (*ProductRecord, error) {
	s, err := u.repo.FindBySKUExact(ctx, code)
	if err != nil {
		return nil, appErr("NOT_FOUND", "Product not found", 404)
	}

	stk, _ := u.repo.StockBySKUID(ctx, s.ID)
	if stk == nil {
		stk = &domainStock.Stock{}
	}

	pType := "Finished Product"
	if s.Category != "" {
		pType = s.Category
	}

	skuSet := make(map[string]bool)
	skuUpper := strings.ToUpper(strings.TrimSpace(s.SKU))
	if skuUpper != "" {
		skuSet[skuUpper] = true
	}

	mappings, _ := u.repo.TikTokMappingsFor(ctx, skuUpper)
	for _, m := range mappings {
		ttUpper := strings.ToUpper(strings.TrimSpace(m.TikTokSKU))
		if ttUpper != "" {
			skuSet[ttUpper] = true
		}
	}

	candidateSKUs := make([]string, 0, len(skuSet))
	for k := range skuSet {
		candidateSKUs = append(candidateSKUs, k)
	}

	var tiktokReserved int64
	if len(candidateSKUs) > 0 {
		tiktokReserved, _ = u.repo.TikTokPendingFor(ctx, candidateSKUs)
	}

	totalReserved := stk.ReservedQty + int(tiktokReserved)
	available := stk.Quantity - totalReserved
	if available < 0 {
		available = 0
	}

	var bundleAvail *int
	if s.IsBundle {
		items, _ := u.repo.BundleItemsFor(ctx, s.SKU)
		if len(items) == 0 {
			zero := 0
			bundleAvail = &zero
		} else {
			minSets := -1
			for _, bi := range items {
				reqQty := bi.Quantity
				if reqQty <= 0 {
					reqQty = 1
				}
				cStock, _ := u.repo.ComponentAvailabilityFor(ctx, strings.ToUpper(strings.TrimSpace(bi.ComponentSKU)))
				sets := cStock / reqQty
				if minSets == -1 || sets < minSets {
					minSets = sets
				}
			}
			if minSets < 0 {
				minSets = 0
			}
			bundleAvail = &minSets
		}
	}

	var accessories []SKUAccessoryRecord
	if !s.IsBundle {
		accItems, _ := u.repo.AccessoriesForSKU(ctx, s.SKU)
		for _, ai := range accItems {
			accessories = append(accessories, SKUAccessoryRecord{
				ID:           ai.ID,
				SKU:          ai.SKU,
				AccessorySKU: ai.AccessorySKU,
				Quantity:     ai.Quantity,
				Note:         ai.Note,
			})
		}
	}

	usedQty, _ := u.repo.UsedQtyFor(ctx, s.ID, skuUpper)

	rec := &ProductRecord{
		ID:              s.ID,
		SKU:             s.SKU,
		Name:            s.Name,
		Type:            pType,
		Barcode:         s.Barcode,
		BaseUnit:        "ชิ้น",
		RetailPrice:     s.Price,
		WholesalePrice:  s.Price,
		Cost:            s.CostPrice,
		Stock:           stk.Quantity,
		ReservedQty:     totalReserved,
		UsedQty:         int(usedQty),
		Reorder:         s.ReorderPoint,
		IsBundle:        s.IsBundle,
		IsActive:        s.Status == "active",
		Available:       available,
		BundleAvailable: bundleAvail,
		Image:           s.Image,
		Accessories:     accessories,
	}
	return rec, nil
}

// Resolve resolves a batch of upper-cased SKU codes (normalized by the caller)
// against the SKU master, falling back to active Inventory Formulas.
func (u *usecase) Resolve(ctx context.Context, codes []string) ([]ResolvedSKU, error) {
	skus, err := u.repo.ResolveUpperTrim(ctx, codes)
	if err != nil {
		return nil, dbErr500("Failed to resolve SKUs")
	}

	found := make(map[string]domainSKU.SKU, len(skus))
	for _, s := range skus {
		found[strings.ToUpper(strings.TrimSpace(s.SKU))] = s
	}

	// For codes not found in SKU master, check active Inventory Formulas (e.g. FI-01)
	var missingCodes []string
	for _, code := range codes {
		if _, ok := found[code]; !ok {
			missingCodes = append(missingCodes, code)
		}
	}

	foundFormulas := make(map[string]domainFormula.InventoryFormula)
	if len(missingCodes) > 0 {
		formulas, _ := u.repo.FindActiveFormulas(ctx, missingCodes)
		for _, f := range formulas {
			foundFormulas[strings.ToUpper(strings.TrimSpace(f.Code))] = f
		}
	}

	records := make([]ResolvedSKU, 0, len(codes))
	for _, code := range codes {
		if s, ok := found[code]; ok {
			records = append(records, ResolvedSKU{Sku: s.SKU, Id: s.ID, Name: s.Name, Found: true})
		} else if f, ok := foundFormulas[code]; ok {
			records = append(records, ResolvedSKU{Sku: f.Code, Id: f.ID, Name: f.Name, Found: true})
		} else {
			records = append(records, ResolvedSKU{Sku: code, Found: false})
		}
	}

	return records, nil
}

// Accessories returns the accessory bindings for one SKU (optional filter).
func (u *usecase) Accessories(ctx context.Context, sku string) ([]SKUAccessoryRecord, error) {
	items, err := u.repo.AccessoriesByUpperSKU(ctx, sku)
	if err != nil {
		return nil, dbErr500("Failed to fetch SKU accessories: " + err.Error())
	}

	records := make([]SKUAccessoryRecord, len(items))
	for i, item := range items {
		records[i] = SKUAccessoryRecord{
			ID:           item.ID,
			SKU:          item.SKU,
			AccessorySKU: item.AccessorySKU,
			Quantity:     item.Quantity,
			Note:         item.Note,
		}
	}

	return records, nil
}

// productNotFound404 mirrors every resolve failure of the old code: any
// lookup error surfaced as "Product not found" (parse failures included).
func productNotFound404() *appErrors.AppError {
	return appErr("NOT_FOUND", "Product not found", 404)
}

// validateAccessories validates the accessory payload of create/update
// requests with the exact historical Thai messages and error codes.
func (u *usecase) validateAccessories(ctx context.Context, skuCode string, accessories []AccessoryLine) *appErrors.AppError {
	for _, acc := range accessories {
		aSKU := acc.AccessorySKU
		if aSKU == "" && acc.SKU != "" {
			aSKU = acc.SKU
		}
		aSKU = strings.ToUpper(strings.TrimSpace(aSKU))
		if aSKU == "" {
			return appErr("BAD_REQUEST", "Accessory SKU cannot be empty", 400)
		}
		if aSKU == skuCode {
			return appErr("BAD_REQUEST", "สินค้าไม่สามารถผูกตนเองเป็น Accessory ได้", 400)
		}
		qty := acc.Quantity
		if qty <= 0 && acc.Qty > 0 {
			qty = acc.Qty
		}
		if qty <= 0 {
			return appErr("BAD_REQUEST", fmt.Sprintf("จำนวน Accessory สำหรับ SKU %s ต้องมากกว่า 0", aSKU), 400)
		}

		// Validate accessory SKU exists
		exists, _ := u.repo.AccessoryExists(ctx, aSKU)
		if !exists {
			return appErr("ACCESSORY_NOT_FOUND", fmt.Sprintf("ไม่พบ SKU Accessory '%s' ในระบบ", aSKU), 400)
		}
		// Cycle check: accessory cannot have this SKU as its accessory
		cycle, _ := u.repo.AccessoryCycleExists(ctx, aSKU, skuCode)
		if cycle {
			return appErr("ACCESSORY_CYCLE", fmt.Sprintf("เกิด Cycle: SKU %s ผูกกับ %s อยู่แล้ว", aSKU, skuCode), 400)
		}
	}
	return nil
}
