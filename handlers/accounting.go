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

func ListAccountMappings(c *fiber.Ctx) error {
	var mappings []models.AccountMapping
	if err := database.DB.Order("mapping_key").Find(&mappings).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mappings)
}

func UpsertAccountMapping(c *fiber.Ctx) error {
	key := c.Params("key")
	var req struct {
		AccountCode string `json:"accountCode"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil || key == "" || req.AccountCode == "" {
		return c.Status(400).JSON(fiber.Map{"error": "accountCode is required"})
	}
	var account models.Account
	if err := database.DB.Where("code = ? AND is_active = ?", req.AccountCode, true).First(&account).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "active account not found"})
	}
	var mapping models.AccountMapping
	err := database.DB.Where("mapping_key = ?", key).First(&mapping).Error
	if err == gorm.ErrRecordNotFound {
		mapping = models.AccountMapping{MappingKey: key}
	} else if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	mapping.AccountCode, mapping.Description, mapping.IsActive = req.AccountCode, req.Description, true
	if err := database.DB.Save(&mapping).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(mapping)
}

// Default account mapping is centralized here so posting and reporting use one source.
const (
	accountCash             = "1100"
	accountBank             = "1110"
	accountAR               = "1200"
	accountInventory        = "1300"
	accountRevenue          = "4000"
	accountCOGS             = "5000"
	accountSalesReturn      = "5100"
	accountDamageLoss       = "5200"
	accountOperatingExpense = "6000"
	accountVATOutput        = "2100"
)

type postingLine struct {
	AccountCode string
	Debit       float64
	Credit      float64
	SKU         string
	Lot         string
	Channel     string
}

type postingRequest struct {
	Date        string
	SourceType  string
	SourceID    uint
	SourceRef   string
	Description string
	CreatedBy   string
	Lines       []postingLine
}

func validatePostingLines(lines []postingLine) error {
	if len(lines) < 2 {
		return fmt.Errorf("journal requires at least two lines")
	}
	totalDebit := 0.0
	totalCredit := 0.0
	for _, line := range lines {
		if line.AccountCode == "" || line.Debit < 0 || line.Credit < 0 {
			return fmt.Errorf("invalid journal line")
		}
		if (line.Debit > 0 && line.Credit > 0) || (line.Debit == 0 && line.Credit == 0) {
			return fmt.Errorf("journal line must contain either debit or credit")
		}
		totalDebit += line.Debit
		totalCredit += line.Credit
	}
	if totalDebit <= 0 || math.Abs(totalDebit-totalCredit) > 0.005 {
		return fmt.Errorf("journal is not balanced: debit %.2f credit %.2f", totalDebit, totalCredit)
	}
	return nil
}

// postJournal posts exactly one balanced journal per source event.
func postJournal(tx *gorm.DB, req postingRequest) (*models.JournalEntry, error) {
	if req.SourceType == "" || req.SourceID == 0 {
		return nil, fmt.Errorf("journal source is required")
	}
	if err := validatePostingLines(req.Lines); err != nil {
		return nil, err
	}

	var existing models.JournalEntry
	if err := tx.Preload("Lines").Where("source_type = ? AND source_id = ?", req.SourceType, req.SourceID).First(&existing).Error; err == nil {
		return &existing, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	code, err := NextCode(tx, "JE-2026-", &models.JournalEntry{}, "code")
	if err != nil {
		return nil, err
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}
	entry := models.JournalEntry{
		Code: code, Date: req.Date, SourceType: req.SourceType, SourceID: req.SourceID,
		SourceRef: req.SourceRef, Description: req.Description, Status: "Posted",
		CreatedBy: req.CreatedBy, PostedAt: time.Now().Format(time.RFC3339),
	}
	if err := tx.Create(&entry).Error; err != nil {
		return nil, err
	}

	for _, input := range req.Lines {
		var account models.Account
		if err := tx.Where("code = ? AND is_active = ?", input.AccountCode, true).First(&account).Error; err != nil {
			return nil, fmt.Errorf("active account %s not found", input.AccountCode)
		}
		line := models.JournalLine{
			JournalEntryID: entry.ID, AccountID: account.ID, AccountCode: account.Code,
			AccountName: account.Name, Debit: input.Debit, Credit: input.Credit,
			SKU: input.SKU, Lot: input.Lot, Channel: input.Channel,
		}
		if err := tx.Create(&line).Error; err != nil {
			return nil, err
		}
		entry.Lines = append(entry.Lines, line)
	}
	if err := writeAuditLog(tx, req.CreatedBy, "Post", "journal_entry", fmt.Sprint(entry.ID), "", entry.Code, req.SourceRef, entry.SourceRef); err != nil {
		return nil, err
	}
	return &entry, nil
}

func writeAuditLog(tx *gorm.DB, actor, action, entity, entityID, before, after, reason, sourceRef string) error {
	return tx.Create(&models.AuditLog{Actor: actor, Action: action, Entity: entity, EntityID: entityID, Before: before, After: after, Reason: reason, SourceRef: sourceRef, CreatedAt: time.Now().UTC().Format(time.RFC3339)}).Error
}
