package models

import "time"

type TiktokOAuthState struct {
	ID        uint       `gorm:"primaryKey" json:"-"`
	StateHash string     `gorm:"uniqueIndex;size:64;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"-"`
	UsedAt    *time.Time `gorm:"index" json:"-"`
	CreatedAt time.Time  `json:"-"`
}