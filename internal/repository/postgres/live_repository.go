package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainLive "chawy-erp-api/internal/domain/live"

	"gorm.io/gorm"
)

type liveRepository struct {
	db *gorm.DB
}

func NewLiveRepository(db *gorm.DB) domainLive.Repository {
	return &liveRepository{db: db}
}

func (r *liveRepository) CreateSession(ctx context.Context, session *domainLive.LiveSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *liveRepository) UpdateSession(ctx context.Context, session *domainLive.LiveSession) error {
	return r.db.WithContext(ctx).Save(session).Error
}

func (r *liveRepository) GetSessionByID(ctx context.Context, id uint) (*domainLive.LiveSession, error) {
	var session domainLive.LiveSession
	err := r.db.WithContext(ctx).
		Preload("Staff").
		First(&session, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainLive.ErrSessionNotFound
		}
		return nil, err
	}
	return &session, nil
}

func (r *liveRepository) ListSessions(ctx context.Context, filter domainLive.SessionFilter) ([]domainLive.LiveSession, int64, error) {
	var sessions []domainLive.LiveSession
	var total int64

	db := r.db.WithContext(ctx).Model(&domainLive.LiveSession{})

	if filter.Month != "" {
		// filter by month prefix YYYY-MM in live_date
		db = db.Where("live_date LIKE ?", filter.Month+"%")
	}
	if filter.Status != "" && filter.Status != "all" {
		db = db.Where("status = ?", strings.ToUpper(filter.Status))
	}
	if filter.StaffID > 0 {
		db = db.Where("staff_id = ?", filter.StaffID)
	}
	if filter.Platform != "" && filter.Platform != "all" {
		db = db.Where("platform = ?", strings.ToUpper(filter.Platform))
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := db.Preload("Staff").Order("start_datetime DESC, id DESC")
	if filter.Limit > 0 {
		offset := (filter.Page - 1) * filter.Limit
		if offset < 0 {
			offset = 0
		}
		query = query.Offset(offset).Limit(filter.Limit)
	}

	if err := query.Find(&sessions).Error; err != nil {
		return nil, 0, err
	}

	return sessions, total, nil
}

func (r *liveRepository) CheckOverlap(ctx context.Context, staffID uint, start, end time.Time, excludeID uint) (bool, *domainLive.LiveSession, error) {
	var existing domainLive.LiveSession
	query := r.db.WithContext(ctx).
		Model(&domainLive.LiveSession{}).
		Where("staff_id = ? AND status != ?", staffID, domainLive.StatusRejected).
		Where("start_datetime < ? AND end_datetime > ?", end, start)

	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	err := query.Preload("Staff").First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil, nil
		}
		return false, nil, err
	}

	return true, &existing, nil
}

func (r *liveRepository) GetNextSessionNo(ctx context.Context, dateStr string) (string, error) {
	// dateStr format: YYYY-MM-DD
	cleanDate := strings.ReplaceAll(dateStr, "-", "")
	prefix := fmt.Sprintf("LIVE-%s", cleanDate)

	var count int64
	err := r.db.WithContext(ctx).
		Model(&domainLive.LiveSession{}).
		Where("session_no LIKE ?", prefix+"%").
		Count(&count).Error
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%02d", prefix, count+1), nil
}

func (r *liveRepository) CreateContentItem(ctx context.Context, item *domainLive.ContentItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *liveRepository) UpdateContentItem(ctx context.Context, item *domainLive.ContentItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *liveRepository) DeleteContentItem(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Delete(&domainLive.ContentItem{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domainLive.ErrContentItemNotFound
	}
	return nil
}

func (r *liveRepository) GetContentItemByID(ctx context.Context, id uint) (*domainLive.ContentItem, error) {
	var item domainLive.ContentItem
	err := r.db.WithContext(ctx).
		Preload("Host").
		First(&item, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainLive.ErrContentItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *liveRepository) ListContentItems(ctx context.Context, filter domainLive.ContentFilter) ([]domainLive.ContentItem, int64, error) {
	var items []domainLive.ContentItem
	var total int64

	db := r.db.WithContext(ctx).Model(&domainLive.ContentItem{})

	if filter.Kind != "" && filter.Kind != "all" {
		db = db.Where("kind = ?", strings.ToUpper(filter.Kind))
	}
	if filter.Platform != "" && filter.Platform != "all" {
		db = db.Where("LOWER(platform) = ?", strings.ToLower(filter.Platform))
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := db.Preload("Host").Order("id DESC")
	if filter.Limit > 0 {
		offset := (filter.Page - 1) * filter.Limit
		if offset < 0 {
			offset = 0
		}
		query = query.Offset(offset).Limit(filter.Limit)
	}

	if err := query.Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
