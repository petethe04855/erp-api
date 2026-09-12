package live

import (
	"context"
	"time"
)

type Repository interface {
	// Sessions
	CreateSession(ctx context.Context, session *LiveSession) error
	UpdateSession(ctx context.Context, session *LiveSession) error
	GetSessionByID(ctx context.Context, id uint) (*LiveSession, error)
	ListSessions(ctx context.Context, filter SessionFilter) ([]LiveSession, int64, error)
	CheckOverlap(ctx context.Context, staffID uint, start, end time.Time, excludeID uint) (bool, *LiveSession, error)
	GetNextSessionNo(ctx context.Context, date string) (string, error)

	// Content Items
	CreateContentItem(ctx context.Context, item *ContentItem) error
	UpdateContentItem(ctx context.Context, item *ContentItem) error
	DeleteContentItem(ctx context.Context, id uint) error
	GetContentItemByID(ctx context.Context, id uint) (*ContentItem, error)
	ListContentItems(ctx context.Context, filter ContentFilter) ([]ContentItem, int64, error)
}
