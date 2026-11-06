package postgres

import (
	"context"
	"fmt"
	"strings"

	domainSeq "chawy-erp-api/internal/domain/sequence"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SequenceRepository struct {
	db *gorm.DB
}

func NewSequenceRepository(db *gorm.DB) domainSeq.Repository {
	return &SequenceRepository{db: db}
}

func (r *SequenceRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

func (r *SequenceRepository) NextNumber(ctx context.Context, docType string, dateStr string) (int, error) {
	db := r.getDB(ctx)
	docType = strings.ToUpper(strings.TrimSpace(docType))
	dateStr = strings.TrimSpace(dateStr)

	// Use Postgres INSERT ... ON CONFLICT DO UPDATE ... RETURNING last_number
	// or row locking within transaction.
	// For SQLite (in unit tests) vs Postgres (in production), we can do an atomic row lock:
	var seq domainSeq.DocumentSequence
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("document_type = ? AND business_date = ?", docType, dateStr).
		First(&seq).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// First entry for this date and document type
			seq = domainSeq.DocumentSequence{
				DocumentType: docType,
				BusinessDate: dateStr,
				LastNumber:   1,
			}
			if createErr := db.Create(&seq).Error; createErr != nil {
				// Handle potential race if concurrent insert happened right before
				var retrySeq domainSeq.DocumentSequence
				if retryErr := db.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("document_type = ? AND business_date = ?", docType, dateStr).
					First(&retrySeq).Error; retryErr == nil {
					retrySeq.LastNumber++
					if saveErr := db.Save(&retrySeq).Error; saveErr != nil {
						return 0, saveErr
					}
					return retrySeq.LastNumber, nil
				}
				return 0, fmt.Errorf("failed to create sequence: %w", createErr)
			}
			return 1, nil
		}
		return 0, err
	}

	seq.LastNumber++
	if err := db.Save(&seq).Error; err != nil {
		return 0, err
	}

	return seq.LastNumber, nil
}
