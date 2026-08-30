package handlers

import (
	"fmt"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type integrityFinding struct {
	Category, Severity, EntityType, EntityRef, Message, Expected, Actual string
	EntityID                                                             uint
}

// RunIntegrityChecks performs read-only checks and stores findings; it never repairs balances.
func RunIntegrityChecks(triggeredBy string) (models.IntegrityRun, error) {
	now := time.Now()
	run := models.IntegrityRun{Code: fmt.Sprintf("IR-%s", now.Format("20060102-150405.000000000")), StartedAt: now.Format(time.RFC3339), TriggeredBy: triggeredBy, Status: "Running"}
	if err := database.DB.Create(&run).Error; err != nil {
		return run, err
	}
	findings, checked := collectIntegrityFindings(database.DB)
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for _, finding := range findings {
			fingerprint := fmt.Sprintf("%s:%s:%d:%s", finding.Category, finding.EntityType, finding.EntityID, finding.EntityRef)
			var issue models.IntegrityIssue
			err := tx.Where("fingerprint = ?", fingerprint).First(&issue).Error
			if err == gorm.ErrRecordNotFound {
				issue = models.IntegrityIssue{Fingerprint: fingerprint, Category: finding.Category, Severity: finding.Severity, EntityType: finding.EntityType, EntityID: finding.EntityID, EntityRef: finding.EntityRef, FirstSeenAt: now.Format(time.RFC3339), Status: "Open"}
			} else if err != nil {
				return err
			}
			issue.Message, issue.Expected, issue.Actual = finding.Message, finding.Expected, finding.Actual
			issue.LastSeenAt, issue.LastSeenRunID, issue.Occurrences = now.Format(time.RFC3339), run.ID, issue.Occurrences+1
			if issue.Status == "Resolved" {
				issue.Status, issue.ResolvedAt, issue.ResolvedBy, issue.Resolution = "Open", "", "", ""
			}
			if err := tx.Save(&issue).Error; err != nil {
				return err
			}
		}
		run.Status, run.CompletedAt, run.CheckedCount, run.IssueCount = "Completed", time.Now().Format(time.RFC3339), checked, len(findings)
		return tx.Save(&run).Error
	})
	return run, err
}

func collectIntegrityFindings(db *gorm.DB) ([]integrityFinding, int) {
	findings := []integrityFinding{}
	checked := 0
	var products []models.Product
	db.Find(&products)
	for _, product := range products {
		checked++
		var lotQty int64
		db.Model(&models.StockLot{}).Where("sku = ?", product.SKU).Select("COALESCE(SUM(remaining_qty),0)").Scan(&lotQty)
		if product.Stock != int(lotQty) {
			findings = append(findings, integrityFinding{"Stock", "Critical", "product", product.SKU, "Product Stock ไม่เท่ากับผลรวม Stock Lot", fmt.Sprint(product.Stock), fmt.Sprint(lotQty), product.ID})
		}
		var movementNet int64
		db.Model(&models.StockMovement{}).Where("sku = ?", product.SKU).Select("COALESCE(SUM(CASE WHEN type = 'IN' THEN qty ELSE -qty END),0)").Scan(&movementNet)
		if product.Stock != int(movementNet) {
			findings = append(findings, integrityFinding{"StockMovement", "High", "product", product.SKU, "Product Stock ไม่เท่ากับ Movement IN - OUT", fmt.Sprint(product.Stock), fmt.Sprint(movementNet), product.ID})
		}
		var invalidLots []models.StockLot
		db.Where("sku = ? AND remaining_qty > 0 AND (expiry_date = '' OR expiry_date < ?)", product.SKU, time.Now().Format("2006-01-02")).Find(&invalidLots)
		for _, lot := range invalidLots {
			checked++
			category, message := "LotExpiry", "Lot มีวันหมดอายุว่างหรือหมดอายุแล้วแต่ยังมี Stock คงเหลือ"
			actual := lot.ExpiryDate
			if actual == "" {
				actual = "ไม่มีวันหมดอายุ"
			}
			findings = append(findings, integrityFinding{category, "High", "stock_lot", lot.Lot, message, "expiry date ในอนาคต", actual, lot.ID})
		}
	}
	var journals []models.JournalEntry
	db.Preload("Lines").Find(&journals)
	for _, journal := range journals {
		checked++
		debit, credit := 0.0, 0.0
		for _, line := range journal.Lines {
			debit += line.Debit
			credit += line.Credit
		}
		if journalIsUnbalanced(debit, credit) {
			findings = append(findings, integrityFinding{"JournalBalance", "Critical", "journal_entry", journal.Code, "Journal Debit และ Credit ไม่สมดุล", fmt.Sprintf("%.2f", debit), fmt.Sprintf("%.2f", credit), journal.ID})
		}
		if !sourceDocumentExists(db, journal.SourceType, journal.SourceID) {
			findings = append(findings, integrityFinding{"OrphanJournal", "High", "journal_entry", journal.Code, "Journal ไม่มีเอกสารต้นทาง", journal.SourceType, fmt.Sprint(journal.SourceID), journal.ID})
		}
	}
	checkMissingJournals(db, &findings, &checked)
	return findings, checked
}

func journalIsUnbalanced(debit, credit float64) bool { return absFloat(debit-credit) >= .005 }

func sourceDocumentExists(db *gorm.DB, sourceType string, sourceID uint) bool {
	var count int64
	var model any
	switch sourceType {
	case "goods_receipt":
		model = &models.GoodsReceive{}
	case "sales_delivery":
		model = &models.SalesOrder{}
	case "customer_invoice":
		model = &models.Invoice{}
	case "customer_payment":
		model = &models.CustomerPayment{}
	case "sales_return":
		model = &models.StockReturn{}
	case "expense":
		model = &models.Expense{}
	default:
		return false
	}
	db.Model(model).Where("id = ?", sourceID).Count(&count)
	return count > 0
}

func checkMissingJournals(db *gorm.DB, findings *[]integrityFinding, checked *int) {
	check := func(sourceType, entityType string, id uint, ref string) {
		*checked = *checked + 1
		var count int64
		db.Model(&models.JournalEntry{}).Where("source_type = ? AND source_id = ?", sourceType, id).Count(&count)
		if count == 0 {
			*findings = append(*findings, integrityFinding{"MissingJournal", "High", entityType, ref, "Posted document ไม่มี Journal", "1 journal", "0 journals", id})
		}
	}
	var receipts []models.GoodsReceive
	db.Where("grand_total > 0").Find(&receipts)
	for _, item := range receipts {
		check("goods_receipt", "goods_receive", item.ID, item.Code)
	}
	var sales []models.SalesOrder
	db.Where("status = ? AND total_cogs > 0", "Completed").Find(&sales)
	for _, item := range sales {
		check("sales_delivery", "sales_order", item.ID, item.Code)
	}
	var invoices []models.Invoice
	db.Where("amount > 0").Find(&invoices)
	for _, item := range invoices {
		check("customer_invoice", "invoice", item.ID, item.Code)
	}
	var payments []models.CustomerPayment
	db.Find(&payments)
	for _, item := range payments {
		check("customer_payment", "customer_payment", item.ID, item.Code)
	}
	var returns []models.StockReturn
	db.Where("status = ?", "Completed").Find(&returns)
	for _, item := range returns {
		check("sales_return", "stock_return", item.ID, item.Code)
	}
	var expenses []models.Expense
	db.Find(&expenses)
	for _, item := range expenses {
		check("expense", "expense", item.ID, item.Code)
	}
}

func RunIntegrityNow(c *fiber.Ctx) error {
	by := "System"
	if name := c.Locals("name"); name != nil {
		by = name.(string)
	}
	run, err := RunIntegrityChecks(by)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(run)
}

func ResolveIntegrityIssue(c *fiber.Ctx) error {
	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := c.BodyParser(&req); err != nil || req.Resolution == "" {
		return c.Status(400).JSON(fiber.Map{"error": "resolution is required"})
	}
	var issue models.IntegrityIssue
	if err := database.DB.First(&issue, c.Params("id")).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Integrity issue not found"})
	}
	by := "System"
	if name := c.Locals("name"); name != nil {
		by = name.(string)
	}
	issue.Status, issue.ResolvedAt, issue.ResolvedBy, issue.Resolution = "Resolved", time.Now().Format(time.RFC3339), by, req.Resolution
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&issue).Error; err != nil {
			return err
		}
		return tx.Create(&models.AuditEvent{OwnerID: issue.ID, OwnerType: "integrity_issues", Action: "Resolved", By: by, At: getNowStr(), Note: req.Resolution}).Error
	}); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(issue)
}
