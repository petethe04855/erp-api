package handlers

import (
	"fmt"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// Helper to write XLSX stream to client
func streamXlsx(c *fiber.Ctx, filename string, headers []string, writeRows func(sw *excelize.StreamWriter) error) error {
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	f := excelize.NewFile()
	defer f.Close()

	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create stream writer: " + err.Error()})
	}

	// Write header row
	headerRow := make([]interface{}, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}
	if err := sw.SetRow("A1", headerRow); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to write header: " + err.Error()})
	}

	// Call row writer
	if err := writeRows(sw); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to write data: " + err.Error()})
	}

	if err := sw.Flush(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to flush stream: " + err.Error()})
	}

	// Stream file to client response writer
	if err := f.Write(c.Response().BodyWriter()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to stream file: " + err.Error()})
	}

	return nil
}

// GET /api/export/sales-orders
func ExportSalesOrders(c *fiber.Ctx) error {
	headers := []string{"Order ID", "Customer", "Date", "Amount (THB)", "Status", "Channel", "Items Count"}
	filename := fmt.Sprintf("sales-orders-export-%s.xlsx", time.Now().Format("2006-01-02"))

	return streamXlsx(c, filename, headers, func(sw *excelize.StreamWriter) error {
		rows, err := database.DB.Model(&models.SalesOrder{}).Order("date desc, id desc").Rows()
		if err != nil {
			return err
		}
		defer rows.Close()

		rowIdx := 2
		for rows.Next() {
			var so models.SalesOrder
			if err := database.DB.ScanRows(rows, &so); err != nil {
				return err
			}

			cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
			rowData := []interface{}{
				so.ID,
				so.Customer,
				so.Date,
				so.Amount,
				so.Status,
				so.Channel,
				so.Items,
			}
			if err := sw.SetRow(cell, rowData); err != nil {
				return err
			}
			rowIdx++
		}
		return nil
	})
}

// GET /api/export/invoices
func ExportInvoices(c *fiber.Ctx) error {
	headers := []string{"Invoice ID", "SO Ref", "Customer", "Issue Date", "Due Date", "Amount (THB)", "Paid (THB)", "Status"}
	filename := fmt.Sprintf("invoices-export-%s.xlsx", time.Now().Format("2006-01-02"))

	return streamXlsx(c, filename, headers, func(sw *excelize.StreamWriter) error {
		rows, err := database.DB.Model(&models.Invoice{}).Order("issue_date desc, id desc").Rows()
		if err != nil {
			return err
		}
		defer rows.Close()

		rowIdx := 2
		for rows.Next() {
			var inv models.Invoice
			if err := database.DB.ScanRows(rows, &inv); err != nil {
				return err
			}

			cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
			rowData := []interface{}{
				inv.ID,
				inv.SoRef,
				inv.Customer,
				inv.IssueDate,
				inv.DueDate,
				inv.Amount,
				inv.Paid,
				inv.Status,
			}
			if err := sw.SetRow(cell, rowData); err != nil {
				return err
			}
			rowIdx++
		}
		return nil
	})
}

// GET /api/export/returns
func ExportReturns(c *fiber.Ctx) error {
	headers := []string{"Return ID", "SO Ref", "SKU", "Product Name", "Qty", "Condition", "Reason", "Date", "Returned By", "Refunded"}
	filename := fmt.Sprintf("stock-returns-export-%s.xlsx", time.Now().Format("2006-01-02"))

	return streamXlsx(c, filename, headers, func(sw *excelize.StreamWriter) error {
		rows, err := database.DB.Model(&models.StockReturn{}).Order("date desc").Rows()
		if err != nil {
			return err
		}
		defer rows.Close()

		rowIdx := 2
		for rows.Next() {
			var ret models.StockReturn
			if err := database.DB.ScanRows(rows, &ret); err != nil {
				return err
			}

			cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
			rowData := []interface{}{
				ret.ID,
				ret.SoRef,
				ret.SKU,
				ret.SkuName,
				ret.Qty,
				ret.Condition,
				ret.Reason,
				ret.Date,
				ret.ReturnedBy,
				ret.Refunded,
			}
			if err := sw.SetRow(cell, rowData); err != nil {
				return err
			}
			rowIdx++
		}
		return nil
	})
}

// GET /api/export/purchase-orders
func ExportPurchaseOrders(c *fiber.Ctx) error {
	headers := []string{"PO ID", "Supplier", "ETA Date", "Date", "Total Cost (THB)", "Status", "PR Ref"}
	filename := fmt.Sprintf("purchase-orders-export-%s.xlsx", time.Now().Format("2006-01-02"))

	return streamXlsx(c, filename, headers, func(sw *excelize.StreamWriter) error {
		rows, err := database.DB.Model(&models.PurchaseOrder{}).Order("date desc, id desc").Rows()
		if err != nil {
			return err
		}
		defer rows.Close()

		rowIdx := 2
		for rows.Next() {
			var po models.PurchaseOrder
			if err := database.DB.ScanRows(rows, &po); err != nil {
				return err
			}

			cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
			rowData := []interface{}{
				po.ID,
				po.Supplier,
				po.EtaDate,
				po.Date,
				po.TotalCost,
				po.Status,
				po.PrRef,
			}
			if err := sw.SetRow(cell, rowData); err != nil {
				return err
			}
			rowIdx++
		}
		return nil
	})
}

// GET /api/export/expenses
func ExportExpenses(c *fiber.Ctx) error {
	headers := []string{"Expense ID", "Date", "Category", "Channel", "Amount (THB)", "Description", "Vendor", "Invoice Ref", "Created By"}
	filename := fmt.Sprintf("expenses-export-%s.xlsx", time.Now().Format("2006-01-02"))

	return streamXlsx(c, filename, headers, func(sw *excelize.StreamWriter) error {
		rows, err := database.DB.Model(&models.Expense{}).Order("date desc").Rows()
		if err != nil {
			return err
		}
		defer rows.Close()

		rowIdx := 2
		for rows.Next() {
			var exp models.Expense
			if err := database.DB.ScanRows(rows, &exp); err != nil {
				return err
			}

			cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
			rowData := []interface{}{
				exp.ID,
				exp.Date,
				exp.Category,
				exp.Channel,
				exp.Amount,
				exp.Description,
				exp.Vendor,
				exp.InvoiceRef,
				exp.CreatedBy,
			}
			if err := sw.SetRow(cell, rowData); err != nil {
				return err
			}
			rowIdx++
		}
		return nil
	})
}

// GET /api/export/pl
func ExportPL(c *fiber.Ctx) error {
	selectedMonth := c.Query("month") // e.g. "2026-05"
	if selectedMonth == "" {
		selectedMonth = time.Now().Format("2006-01")
	}

	headers := []string{"P&L Line Item", "Amount (THB)", "Percentage"}
	filename := fmt.Sprintf("pl-report-%s.xlsx", selectedMonth)

	return streamXlsx(c, filename, headers, func(sw *excelize.StreamWriter) error {
		// Calculate Revenue
		var soRevenue float64
		database.DB.Model(&models.SalesOrder{}).
			Where("date LIKE ? AND status = 'Completed'", selectedMonth+"%").
			Select("COALESCE(SUM(amount), 0)").Scan(&soRevenue)

		var moRevenue float64
		database.DB.Model(&models.ManualOrder{}).
			Where("date LIKE ? AND status = 'Completed'", selectedMonth+"%").
			Select("COALESCE(SUM(amount), 0)").Scan(&moRevenue)

		var ttRevenue float64
		database.DB.Model(&models.TiktokOrder{}).
			Where("date LIKE ? AND settled = true", selectedMonth+"%").
			Select("COALESCE(SUM(net_revenue), 0)").Scan(&ttRevenue)

		totalRevenue := soRevenue + moRevenue + ttRevenue

		// Calculate COGS
		var cogs float64
		database.DB.Model(&models.Expense{}).
			Where("date LIKE ? AND category = 'COGS/วัตถุดิบ'", selectedMonth+"%").
			Select("COALESCE(SUM(amount), 0)").Scan(&cogs)

		// Calculate OpEx
		var opex float64
		database.DB.Model(&models.Expense{}).
			Where("date LIKE ? AND category != 'COGS/วัตถุดิบ'", selectedMonth+"%").
			Select("COALESCE(SUM(amount), 0)").Scan(&opex)

		grossProfit := totalRevenue - cogs
		netProfit := grossProfit - opex

		// Write rows
		items := []struct {
			Label string
			Val   float64
		}{
			{"Total Revenue (Completed Orders + TikTok Net)", totalRevenue},
			{"  - Sales Orders (Manual/Website)", soRevenue},
			{"  - Manual Orders", moRevenue},
			{"  - TikTok Net Revenue", ttRevenue},
			{"Cost of Goods Sold (COGS)", cogs},
			{"Gross Profit", grossProfit},
			{"Operating Expenses (OpEx)", opex},
			{"Net Profit", netProfit},
		}

		for idx, item := range items {
			cell, _ := excelize.CoordinatesToCellName(1, idx+2)
			var pctStr string
			if totalRevenue > 0 {
				pctStr = fmt.Sprintf("%.1f%%", (item.Val/totalRevenue)*100)
			} else {
				pctStr = "0.0%"
			}

			rowData := []interface{}{
				item.Label,
				item.Val,
				pctStr,
			}
			if err := sw.SetRow(cell, rowData); err != nil {
				return err
			}
		}
		return nil
	})
}

// GET /api/export/budget
func ExportBudget(c *fiber.Ctx) error {
	selectedMonth := c.Query("month") // e.g. "2026-05"
	if selectedMonth == "" {
		selectedMonth = time.Now().Format("2006-01")
	}

	headers := []string{"Category", "Channel", "Budget Amount", "Actual Spend", "Usage %", "Variance"}
	filename := fmt.Sprintf("budget-utilization-%s.xlsx", selectedMonth)

	// Parse Year/Month
	var year, month int
	fmt.Sscanf(selectedMonth, "%d-%d", &year, &month)

	return streamXlsx(c, filename, headers, func(sw *excelize.StreamWriter) error {
		var budgets []models.MonthBudget
		if err := database.DB.Where("year = ? AND month = ?", year, month).Find(&budgets).Error; err != nil {
			return err
		}

		rowIdx := 2
		for _, b := range budgets {
			var actual float64
			database.DB.Model(&models.Expense{}).
				Where("date LIKE ? AND category = ? AND channel = ?", selectedMonth+"%", b.Category, b.Channel).
				Select("COALESCE(SUM(amount), 0)").
				Scan(&actual)

			var pctStr string
			if b.BudgetAmount > 0 {
				pctStr = fmt.Sprintf("%.1f%%", (actual/b.BudgetAmount)*100)
			} else {
				pctStr = "0.0%"
			}
			variance := b.BudgetAmount - actual

			cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
			rowData := []interface{}{
				b.Category,
				b.Channel,
				b.BudgetAmount,
				actual,
				pctStr,
				variance,
			}
			if err := sw.SetRow(cell, rowData); err != nil {
				return err
			}
			rowIdx++
		}
		return nil
	})
}

// GET /api/export/tiktok-orders
func ExportTiktokOrders(c *fiber.Ctx) error {
	headers := []string{"Order ID", "Product", "SKU", "Qty", "Gross Amount (THB)", "Net Revenue (THB)", "Platform Fee (THB)", "Settlement Ref", "Status"}
	filename := fmt.Sprintf("tiktok-orders-export-%s.xlsx", time.Now().Format("2006-01-02"))

	return streamXlsx(c, filename, headers, func(sw *excelize.StreamWriter) error {
		rows, err := database.DB.Model(&models.TiktokOrder{}).Order("id desc").Rows()
		if err != nil {
			return err
		}
		defer rows.Close()

		rowIdx := 2
		for rows.Next() {
			var order models.TiktokOrder
			if err := database.DB.ScanRows(rows, &order); err != nil {
				return err
			}

			cell, _ := excelize.CoordinatesToCellName(1, rowIdx)
			rowData := []interface{}{
				order.ID,
				order.Product,
				order.SKU,
				order.Qty,
				order.Amount,
				order.NetRevenue,
				order.PlatformFee,
				order.SettlementRef,
				order.Status,
			}
			if err := sw.SetRow(cell, rowData); err != nil {
				return err
			}
			rowIdx++
		}
		return nil
	})
}
