package models

// IntegrityRun records one complete, read-only integrity scan.
type IntegrityRun struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code         string `gorm:"uniqueIndex;not null" json:"code"`
	StartedAt    string `json:"startedAt"`
	CompletedAt  string `json:"completedAt"`
	TriggeredBy  string `json:"triggeredBy"`
	Status       string `json:"status"`
	CheckedCount int    `json:"checkedCount"`
	IssueCount   int    `json:"issueCount"`
}

// IntegrityIssue is a persistent finding; scans never alter business balances.
type IntegrityIssue struct {
	ID            uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Fingerprint   string `gorm:"size:255;uniqueIndex;not null" json:"fingerprint"`
	Category      string `gorm:"index" json:"category"`
	Severity      string `gorm:"index" json:"severity"`
	EntityType    string `gorm:"index" json:"entityType"`
	EntityID      uint   `gorm:"index" json:"entityId"`
	EntityRef     string `gorm:"index" json:"entityRef"`
	Message       string `json:"message"`
	Expected      string `json:"expected"`
	Actual        string `json:"actual"`
	Status        string `gorm:"index" json:"status"`
	FirstSeenAt   string `json:"firstSeenAt"`
	LastSeenAt    string `json:"lastSeenAt"`
	LastSeenRunID uint   `gorm:"index" json:"lastSeenRunId"`
	Occurrences   int    `json:"occurrences"`
	ResolvedAt    string `json:"resolvedAt"`
	ResolvedBy    string `json:"resolvedBy"`
	Resolution    string `json:"resolution"`
}
