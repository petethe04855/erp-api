package sequence

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainSeq "chawy-erp-api/internal/domain/sequence"
)

var bangkokLoc *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		loc = time.FixedZone("Asia/Bangkok", 7*60*60)
	}
	bangkokLoc = loc
}

func BangkokLocation() *time.Location {
	return bangkokLoc
}

type Usecase interface {
	// Generate produces a standardized document number: PREFIX-YYYY-MM-DD-NNN
	Generate(ctx context.Context, docType string, businessDate *time.Time) (string, error)
	// FormatNumber formats a prefix, date and sequence number into PREFIX-YYYY-MM-DD-NNN
	FormatNumber(docType string, dateStr string, num int) string
}

type sequenceUsecase struct {
	repo domainSeq.Repository
}

func NewSequenceUsecase(repo domainSeq.Repository) Usecase {
	return &sequenceUsecase{repo: repo}
}

func (u *sequenceUsecase) Generate(ctx context.Context, docType string, businessDate *time.Time) (string, error) {
	prefix := strings.ToUpper(strings.TrimSpace(docType))
	switch prefix {
	case string(domainSeq.TypeGoodsReceive),
		string(domainSeq.TypeGoodsIssue),
		string(domainSeq.TypeSalesOrder),
		string(domainSeq.TypeInvoice),
		string(domainSeq.TypeQuotation),
		string(domainSeq.TypeSalesReturn),
		string(domainSeq.TypePurchaseOrder):
		// valid prefix
	default:
		return "", fmt.Errorf("unsupported document type: %s", docType)
	}

	targetTime := time.Now().In(bangkokLoc)
	if businessDate != nil && !businessDate.IsZero() {
		targetTime = businessDate.In(bangkokLoc)
	}

	dateStr := targetTime.Format("2006-01-02")
	nextNum, err := u.repo.NextNumber(ctx, prefix, dateStr)
	if err != nil {
		return "", fmt.Errorf("failed to generate document sequence for %s on %s: %w", prefix, dateStr, err)
	}

	return u.FormatNumber(prefix, dateStr, nextNum), nil
}

func (u *sequenceUsecase) FormatNumber(docType string, dateStr string, num int) string {
	if num <= 999 {
		return fmt.Sprintf("%s-%s-%03d", docType, dateStr, num)
	}
	return fmt.Sprintf("%s-%s-%d", docType, dateStr, num)
}
