package finance

import (
	"time"
)

type JournalStatus string

const (
	JournalStatusPosted   JournalStatus = "Posted"
	JournalStatusReversed JournalStatus = "Reversed"
)

// JournalEntry is an immutable posted accounting document.
type JournalEntry struct {
	ID           uint          `gorm:"primaryKey;autoIncrement" json:"id"`
	Code         string        `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Date         string        `gorm:"size:20;index;not null" json:"date"`
	SourceType   string        `gorm:"size:50;uniqueIndex:idx_journal_source,priority:1;not null" json:"source_type"`
	SourceID     uint          `gorm:"uniqueIndex:idx_journal_source,priority:2;not null" json:"source_id"`
	SourceRef    string        `gorm:"size:100;index" json:"source_ref"`
	Description  string        `gorm:"size:255" json:"description"`
	Status       JournalStatus `gorm:"size:30;default:'Posted'" json:"status"`
	ReversalOfID *uint         `gorm:"index" json:"reversal_of_id,omitempty"`
	CreatedBy    string        `gorm:"size:100" json:"created_by"`
	PostedAt     time.Time     `json:"posted_at"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`

	Lines []JournalLine `gorm:"foreignKey:JournalEntryID;constraint:OnDelete:CASCADE" json:"lines"`
}

// JournalLine is one debit or credit leg of a journal entry.
type JournalLine struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	JournalEntryID uint      `gorm:"index;not null" json:"journal_entry_id"`
	AccountID      uint      `gorm:"index;not null" json:"account_id"`
	AccountCode    string    `gorm:"size:20;index;not null" json:"account_code"`
	AccountName    string    `gorm:"size:150" json:"account_name"`
	Debit          float64   `gorm:"type:numeric(15,2);default:0" json:"debit"`
	Credit         float64   `gorm:"type:numeric(15,2);default:0" json:"credit"`
	SKU            string    `gorm:"size:50;index" json:"sku,omitempty"`
	Lot            string    `gorm:"size:50" json:"lot,omitempty"`
	Channel        string    `gorm:"size:50;index" json:"channel,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
