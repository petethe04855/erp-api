package catalog

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	"time"
)

// Import reproduces the workspace ImportProductsXLSX row processing:
// per-row transactions (all-or-nothing per row, not per file), in-file
// duplicate tracking, DB duplicate rejection and the historical Thai messages.
func (u *usecase) Import(ctx context.Context, totalRows int, lines []ImportLine) *ImportSummary {
	summary := &ImportSummary{
		TotalRows: totalRows,
		Errors:    make([]ImportRowError, 0),
	}

	// Track SKUs to prevent duplicates within the Excel file
	seenSKU := make(map[string]int)

	for _, line := range lines {
		rowNum := line.Row
		skuCode := strings.ToUpper(line.SKU)
		name := line.Name
		barcode := line.Barcode
		category := line.Category
		priceStr := line.Price
		costStr := line.Cost
		stockStr := line.Stock
		reorderStr := line.Reorder

		// 1. Check SKU is not empty
		if skuCode == "" {
			summary.Failed++
			summary.Errors = append(summary.Errors, ImportRowError{
				Row:   rowNum,
				SKU:   skuCode,
				Error: "รหัส SKU ไม่สามารถเป็นค่าว่างได้",
			})
			continue
		}

		// 2. Check SKU is not duplicate within Excel file
		if firstRow, exists := seenSKU[skuCode]; exists {
			summary.Failed++
			summary.Errors = append(summary.Errors, ImportRowError{
				Row:   rowNum,
				SKU:   skuCode,
				Error: fmt.Sprintf("รหัส SKU ซ้ำกับแถวที่ %d ในไฟล์ Excel", firstRow),
			})
			continue
		}
		seenSKU[skuCode] = rowNum

		// 3. Check SKU does not already exist in database (ห้ามซ้ำกับในระบบ)
		exists, _ := u.repo.ExistsUpper(ctx, skuCode)
		if exists {
			summary.Failed++
			summary.Errors = append(summary.Errors, ImportRowError{
				Row:   rowNum,
				SKU:   skuCode,
				Error: fmt.Sprintf("รหัส SKU '%s' มีอยู่ในระบบแล้ว (ห้ามซ้ำ)", skuCode),
			})
			continue
		}

		// 4. Check Product Name is not empty
		if name == "" {
			summary.Failed++
			summary.Errors = append(summary.Errors, ImportRowError{
				Row:   rowNum,
				SKU:   skuCode,
				Error: "ชื่อสินค้าไม่สามารถเป็นค่าว่างได้",
			})
			continue
		}

		if category == "" {
			category = "Finished Product"
		}

		price, _ := strconv.ParseFloat(priceStr, 64)
		cost, _ := strconv.ParseFloat(costStr, 64)
		initialStock, _ := strconv.Atoi(stockStr)
		if initialStock < 0 {
			initialStock = 0
		}
		reorder, _ := strconv.Atoi(reorderStr)
		if reorder < 0 {
			reorder = 10
		}

		// Perform Creation inside a per-row transaction
		txErr := u.txMgr.Transaction(ctx, func(txCtx context.Context) error {
			// Double check uniqueness with lock
			dbExists, _ := u.repo.ExistsUpper(txCtx, skuCode)
			if dbExists {
				return fmt.Errorf("รหัส SKU '%s' มีอยู่ในระบบแล้ว", skuCode)
			}

			// New SKU -> Create SKU + Stock + Movement (if stock > 0)
			newSKU := domainSKU.SKU{
				SKU:          skuCode,
				Name:         name,
				Barcode:      barcode,
				Category:     category,
				Price:        price,
				CostPrice:    cost,
				IsBundle:     false,
				Status:       "active",
				ReorderPoint: reorder,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}

			if err := u.repo.CreateSKU(txCtx, &newSKU); err != nil {
				return err
			}

			// Initial Stock
			stockEntity := domainStock.Stock{
				SKUID:        newSKU.ID,
				SKUCode:      newSKU.SKU,
				WarehouseID:  1,
				Quantity:     initialStock,
				AvailableQty: initialStock,
				UpdatedAt:    time.Now(),
			}
			if err := u.repo.CreateStock(txCtx, &stockEntity); err != nil {
				return err
			}

			if initialStock > 0 {
				movement := domainStock.StockMovement{
					SKUID:         newSKU.ID,
					SKUCode:       newSKU.SKU,
					WarehouseID:   1,
					Type:          domainStock.MovementIn,
					Quantity:      initialStock,
					BeforeQty:     0,
					AfterQty:      initialStock,
					ReferenceType: "opening_stock",
					ReferenceID:   newSKU.SKU,
					Note:          "Initial stock from XLSX import",
					CreatedAt:     time.Now(),
				}
				if err := u.repo.CreateMovement(txCtx, &movement); err != nil {
					return err
				}
			}

			summary.Created++
			return nil
		})

		if txErr != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, ImportRowError{
				Row:   rowNum,
				SKU:   skuCode,
				Error: txErr.Error(),
			})
		}
	}

	return summary
}
