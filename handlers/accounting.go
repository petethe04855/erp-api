package handlers

import (
	"fmt"
	"math"
	"time"

	"chawy-erp-api/models"
	"gorm.io/gorm"
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
	return &entry, nil
}
