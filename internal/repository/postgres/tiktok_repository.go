package postgres

import (
	"context"
	"errors"
	"time"

	"chawy-erp-api/internal/domain/tiktok"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TikTokRepository struct {
	db *gorm.DB
}

func NewTikTokRepository(db *gorm.DB) tiktok.Repository {
	return &TikTokRepository{db: db}
}

func (r *TikTokRepository) GetConnection(ctx context.Context) (*tiktok.TiktokConnection, error) {
	var conn tiktok.TiktokConnection
	err := r.db.WithContext(ctx).First(&conn, 1).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conn, nil
}

func (r *TikTokRepository) SaveConnection(ctx context.Context, conn *tiktok.TiktokConnection) error {
	conn.ID = 1
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"access_token", "refresh_token", "access_token_expires_at",
			"refresh_token_expires_at", "shop_cipher", "seller_name",
			"seller_base_region", "granted_scopes", "updated_at",
		}),
	}).Create(conn).Error
}

func (r *TikTokRepository) CreateOAuthState(ctx context.Context, state *tiktok.TiktokOAuthState) error {
	// Clean expired states first
	_ = r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&tiktok.TiktokOAuthState{}).Error
	return r.db.WithContext(ctx).Create(state).Error
}

func (r *TikTokRepository) ValidateAndConsumeOAuthState(ctx context.Context, stateHash string) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&tiktok.TiktokOAuthState{}).
		Where("state_hash = ? AND expires_at > ? AND used_at IS NULL", stateHash, now).
		Update("used_at", now)

	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func (r *TikTokRepository) IsWebhookEventRecorded(ctx context.Context, eventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&tiktok.TiktokWebhookEvent{}).
		Where("event_id = ?", eventID).
		Count(&count).Error
	return count > 0, err
}

func (r *TikTokRepository) RecordWebhookEvent(ctx context.Context, event *tiktok.TiktokWebhookEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *TikTokRepository) UpsertOrders(ctx context.Context, orders []tiktok.TiktokOrder) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range orders {
			order := &orders[i]
			if err := tx.Omit("Items").Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"date", "product", "sku", "qty", "amount", "status",
				}),
			}).Create(order).Error; err != nil {
				return err
			}

			// Replace Line Items cleanly
			if err := tx.Where("order_id = ?", order.ID).Delete(&tiktok.TiktokOrderItem{}).Error; err != nil {
				return err
			}
			if len(order.Items) > 0 {
				if err := tx.Create(&order.Items).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *TikTokRepository) GetOrderByID(ctx context.Context, id string) (*tiktok.TiktokOrder, error) {
	var order tiktok.TiktokOrder
	err := r.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}

func (r *TikTokRepository) UpdateOrderStockDeducted(ctx context.Context, orderID string, deducted bool) error {
	return r.db.WithContext(ctx).Model(&tiktok.TiktokOrder{}).
		Where("id = ?", orderID).
		Update("stock_deducted", deducted).Error
}

func (r *TikTokRepository) ListRecentOrders(ctx context.Context, limit int) ([]tiktok.TiktokOrder, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var orders []tiktok.TiktokOrder
	err := r.db.WithContext(ctx).Preload("Items").Order("date DESC").Limit(limit).Find(&orders).Error
	return orders, err
}

func (r *TikTokRepository) CreateSyncRun(ctx context.Context, run *tiktok.TiktokSyncRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *TikTokRepository) UpdateSyncRun(ctx context.Context, run *tiktok.TiktokSyncRun) error {
	return r.db.WithContext(ctx).Save(run).Error
}

func (r *TikTokRepository) ListSyncRuns(ctx context.Context, limit int) ([]tiktok.TiktokSyncRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var runs []tiktok.TiktokSyncRun
	err := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit).Find(&runs).Error
	return runs, err
}

func (r *TikTokRepository) SaveMapping(ctx context.Context, m *tiktok.SKUMapping) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tiktok_sku"}},
		DoUpdates: clause.AssignmentColumns([]string{"erp_sku", "updated_at"}),
	}).Create(m).Error
}

func (r *TikTokRepository) GetMapping(ctx context.Context, tiktokSKU string) (*tiktok.SKUMapping, error) {
	var m tiktok.SKUMapping
	err := r.db.WithContext(ctx).Where("tiktok_sku = ?", tiktokSKU).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *TikTokRepository) ListMappings(ctx context.Context) ([]tiktok.SKUMapping, error) {
	var items []tiktok.SKUMapping
	err := r.db.WithContext(ctx).Order("id DESC").Find(&items).Error
	return items, err
}

func (r *TikTokRepository) CreateSyncLog(ctx context.Context, l *tiktok.SyncLog) error {
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *TikTokRepository) GetSyncLogs(ctx context.Context, limit int) ([]tiktok.SyncLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var logs []tiktok.SyncLog
	err := r.db.WithContext(ctx).Order("id DESC").Limit(limit).Find(&logs).Error
	return logs, err
}
