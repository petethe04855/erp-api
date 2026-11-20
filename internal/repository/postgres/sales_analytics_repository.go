package postgres

import (
	"context"
	"sort"
	"strings"
	"time"

	"chawy-erp-api/internal/domain/salesanalytics"
	"chawy-erp-api/internal/domain/shopee"
	"chawy-erp-api/internal/domain/tiktok"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
)

// bangkok is the business time zone of every order fact.
var bangkok = time.FixedZone("Asia/Bangkok", 7*60*60)

// cancelledStatuses never count towards sales.
var cancelledStatuses = []string{"CANCELLED", "RETURNED", "REFUNDED"}

// tiktokFulfilledStatuses mirrors the province report's default filter so the
// dashboard and the geographic report agree on which orders count.
var tiktokFulfilledStatuses = []string{
	"AWAITING_COLLECTION", "IN_TRANSIT", "DELIVERED", "COMPLETED", "SHIPPED",
}

// SalesAnalyticsRepository serves the CRM sales dashboard facts from the
// marketplace order tables imported by the Shopee and TikTok sync jobs.
type SalesAnalyticsRepository struct {
	db *gorm.DB
}

// NewSalesAnalyticsRepository builds the repository.
func NewSalesAnalyticsRepository(db *gorm.DB) *SalesAnalyticsRepository {
	return &SalesAnalyticsRepository{db: db}
}

func (r *SalesAnalyticsRepository) handle(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

// SalesOrders returns every accepted order of the range, oldest first. The same
// order id may appear under both platforms; the caller keys on platform+id.
func (r *SalesAnalyticsRepository) SalesOrders(ctx context.Context, q salesanalytics.Query) ([]salesanalytics.Order, error) {
	channel := strings.ToLower(strings.TrimSpace(q.Channel))
	if channel == "" {
		channel = salesanalytics.ChannelAll
	}

	out := []salesanalytics.Order{}
	if channel == salesanalytics.ChannelAll || channel == salesanalytics.ChannelShopee {
		rows, err := r.shopeeOrders(ctx, q)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	if channel == salesanalytics.ChannelAll || strings.HasPrefix(channel, salesanalytics.ChannelTikTok) {
		rows, err := r.tiktokOrders(ctx, q)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OrderedAt != out[j].OrderedAt {
			return out[i].OrderedAt < out[j].OrderedAt
		}
		return out[i].OrderNo < out[j].OrderNo
	})
	return out, nil
}

// shopeeOrders loads Shopee orders and their line items.
func (r *SalesAnalyticsRepository) shopeeOrders(ctx context.Context, q salesanalytics.Query) ([]salesanalytics.Order, error) {
	db := r.handle(ctx).Model(&shopee.ShopeeOrder{}).Preload("Items")

	if from, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(q.DateFrom), bangkok); err == nil {
		db = db.Where("order_date >= ?", from)
	}
	if to, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(q.DateTo), bangkok); err == nil {
		db = db.Where("order_date < ?", to.AddDate(0, 0, 1))
	}
	db = db.Where("status NOT IN ?", cancelledStatuses)

	var rows []shopee.ShopeeOrder
	if err := db.Order("order_date ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]salesanalytics.Order, 0, len(rows))
	for _, o := range rows {
		at := o.OrderDate.In(bangkok)
		fact := salesanalytics.Order{
			Platform:    salesanalytics.ChannelShopee,
			OrderNo:     o.ID,
			OrderedAt:   at.Format("2006-01-02 15:04:05"),
			HasTime:     true,
			BuyerID:     o.BuyerUsername,
			Buyer:       o.BuyerUsername,
			Date:        at.Format("2006-01-02"),
			Channel:     "Shopee",
			ChannelType: salesanalytics.ChannelShopee,
			Province:    o.Province,
			Sets:        []string{},
			Items:       []string{},
			Lines:       []salesanalytics.Line{},
		}
		for _, it := range o.Items {
			fact.Amount += it.SalePrice
			fact.Items = append(fact.Items, it.ProductName)
			fact.Lines = append(fact.Lines, salesanalytics.Line{
				SKU: it.SKU, Name: it.ProductName, Qty: it.Qty, Amount: it.SalePrice,
			})
		}
		out = append(out, fact)
	}
	return out, nil
}

// tiktokOrders loads TikTok orders and their line items. TikTok exposes no
// buyer username, only an opaque user_id (stored by the order sync since
// 2026-11): it fills BuyerID and Buyer so repeat buyers group together. Orders
// synced before then, or imported from files, have no buyer.
func (r *SalesAnalyticsRepository) tiktokOrders(ctx context.Context, q salesanalytics.Query) ([]salesanalytics.Order, error) {
	db := r.handle(ctx).Model(&tiktok.TiktokOrder{}).Preload("Items")

	// date is stored as "YYYY-MM-DD[ HH:MM:SS]" text, so string comparison is
	// the range filter.
	if from := strings.TrimSpace(q.DateFrom); from != "" {
		db = db.Where("date >= ?", from)
	}
	if to := strings.TrimSpace(q.DateTo); to != "" {
		db = db.Where("date <= ?", to+" 23:59:59")
	}
	db = db.Where("status IN ?", tiktokFulfilledStatuses)

	var rows []tiktok.TiktokOrder
	if err := db.Order("date ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]salesanalytics.Order, 0, len(rows))
	for _, o := range rows {
		stamp := strings.TrimSpace(o.Date)
		day := stamp
		if len(day) > 10 {
			day = day[:10]
		}

		fact := salesanalytics.Order{
			Platform:    salesanalytics.ChannelTikTok,
			OrderNo:     o.ID,
			OrderedAt:   stamp,
			HasTime:     len(stamp) > 10,
			Date:        day,
			Amount:      o.Amount,
			BuyerID:     o.BuyerUserID,
			Buyer:       o.BuyerUserID,
			Channel:     "TikTok",
			ChannelType: salesanalytics.ChannelTikTok,
			Province:    o.RecipientProvince,
			Sets:        []string{},
			Items:       []string{},
			Lines:       []salesanalytics.Line{},
		}
		for _, it := range o.Items {
			fact.Items = append(fact.Items, it.ProductName)
			fact.Lines = append(fact.Lines, salesanalytics.Line{
				SKU: it.SKU, Name: it.ProductName, Qty: it.Qty, Amount: it.Amount,
			})
		}
		if len(fact.Lines) == 0 && (o.SKU != "" || o.Product != "") {
			fact.Items = append(fact.Items, o.Product)
			fact.Lines = append(fact.Lines, salesanalytics.Line{
				SKU: o.SKU, Name: o.Product, Qty: o.Qty, Amount: o.Amount,
			})
		}
		out = append(out, fact)
	}
	return out, nil
}
