package sequence

import (
	"context"
	"time"
)

type DocumentType string

const (
	TypeGoodsReceive  DocumentType = "GR"
	TypeGoodsIssue    DocumentType = "GI"
	TypeSalesOrder    DocumentType = "SO"
	TypeInvoice       DocumentType = "INV"
	TypeQuotation     DocumentType = "QT"
	TypeSalesReturn   DocumentType = "RT"
	TypePurchaseOrder DocumentType = "PO"
)

// DocumentSequence records the last running sequence number for a document type and business date.
type DocumentSequence struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	DocumentType string    `json:"document_type" gorm:"size:20;not null;uniqueIndex:idx_doc_type_date"`
	BusinessDate string    `json:"business_date" gorm:"size:10;not null;uniqueIndex:idx_doc_type_date"` // YYYY-MM-DD
	LastNumber   int       `json:"last_number" gorm:"not null;default:0"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (DocumentSequence) TableName() string {
	return "document_sequences"
}

type Repository interface {
	// NextNumber gets the next atomic integer sequence for docType and dateStr (YYYY-MM-DD)
	// within a database transaction.
	NextNumber(ctx context.Context, docType string, dateStr string) (int, error)
}
