package postgres

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"chawy-erp-api/internal/domain/crm"
	"chawy-erp-api/internal/domain/shopee"
	"chawy-erp-api/internal/domain/tiktok"
	"chawy-erp-api/pkg/database"
	"chawy-erp-api/pkg/province"

	"gorm.io/gorm"
)

type CRMRepository struct {
	db *gorm.DB
}

func NewCRMRepository(db *gorm.DB) crm.Repository {
	return &CRMRepository{db: db}
}

func (r *CRMRepository) handle(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

type provinceQueryResult struct {
	Province   string  `gorm:"column:province"`
	OrderCount int64   `gorm:"column:order_count"`
	ItemQty    int64   `gorm:"column:item_qty"`
	GrossSales float64 `gorm:"column:gross_sales"`
}

func (r *CRMRepository) GetTiktokProvinceReport(ctx context.Context, query crm.ProvinceQuery) (*crm.TiktokProvinceReport, error) {
	channel := strings.ToLower(strings.TrimSpace(query.Channel))
	if channel == "" {
		channel = "all"
	}

	var results []provinceQueryResult
	if channel == "all" || channel == "tiktok" {
		rows, err := r.queryTiktok(ctx, query)
		if err != nil {
			return nil, err
		}
		results = append(results, rows...)
	}
	if channel == "all" || channel == "shopee" {
		rows, err := r.queryShopee(ctx, query)
		if err != nil {
			return nil, err
		}
		results = append(results, rows...)
	}
	results = mergeProvinceResults(results)

	report := &crm.TiktokProvinceReport{
		Summary: crm.TiktokProvinceSummary{
			TopProvince: "-",
		},
		Provinces: make([]crm.TiktokProvinceRow, 0, len(results)),
	}

	if len(results) == 0 {
		return report, nil
	}

	var totalOrders int64
	var totalItemQty int64
	var totalGrossSales float64
	var validOrders int64
	var validProvinceCount int
	var topValidProvince string
	var topValidCount int64

	for _, res := range results {
		totalOrders += res.OrderCount
		totalItemQty += res.ItemQty
		totalGrossSales += res.GrossSales

		if res.Province != province.UnknownProvince {
			validOrders += res.OrderCount
			validProvinceCount++
			if res.OrderCount > topValidCount {
				topValidCount = res.OrderCount
				topValidProvince = res.Province
			}
		}
	}

	// Set Summary Top Province
	if topValidProvince != "" {
		report.Summary.TopProvince = topValidProvince
	} else if len(results) > 0 {
		report.Summary.TopProvince = results[0].Province
	}

	report.Summary.TotalOrders = totalOrders
	report.Summary.TotalItemQty = totalItemQty
	report.Summary.GrossSales = roundFloat(totalGrossSales, 2)
	report.Summary.ProvinceCount = validProvinceCount

	if totalOrders > 0 {
		completeness := (float64(validOrders) / float64(totalOrders)) * 100
		report.Summary.DataCompletenessPercent = roundFloat(completeness, 2)
	}

	// Calculate individual SharePercent
	for _, res := range results {
		share := 0.0
		if totalOrders > 0 {
			share = (float64(res.OrderCount) / float64(totalOrders)) * 100
		}
		report.Provinces = append(report.Provinces, crm.TiktokProvinceRow{
			Province:     res.Province,
			OrderCount:   res.OrderCount,
			ItemQty:      res.ItemQty,
			GrossSales:   roundFloat(res.GrossSales, 2),
			SharePercent: roundFloat(share, 2),
		})
	}

	return report, nil
}

// queryTiktok aggregates tiktok_orders by recipient province.
func (r *CRMRepository) queryTiktok(ctx context.Context, query crm.ProvinceQuery) ([]provinceQueryResult, error) {
	db := r.handle(ctx).Model(&tiktok.TiktokOrder{})

	// 1. Date Filter
	if query.DateFrom != "" {
		db = db.Where("date >= ?", query.DateFrom)
	}
	if query.DateTo != "" {
		// Include up to end of the specified day
		db = db.Where("date <= ?", query.DateTo+" 23:59:59")
	}

	// 2. Status Filter
	switch strings.ToLower(strings.TrimSpace(query.Status)) {
	case "cancelled":
		db = db.Where("status IN (?)", []string{"CANCELLED", "RETURNED", "REFUNDED"})
	case "all":
		db = db.Where("status != ?", "UNPAID")
	default: // "fulfilled" (default)
		db = db.Where("status IN (?)", []string{"AWAITING_COLLECTION", "IN_TRANSIT", "DELIVERED", "COMPLETED", "SHIPPED"})
	}

	db = applyProvinceFilter(db, "recipient_province", query.Province)

	var results []provinceQueryResult
	err := db.Select(`
		COALESCE(NULLIF(TRIM(recipient_province), ''), '` + province.UnknownProvince + `') AS province,
		COUNT(DISTINCT id) AS order_count,
		COALESCE(SUM(qty), 0) AS item_qty,
		COALESCE(SUM(amount), 0) AS gross_sales
	`).
		Group("COALESCE(NULLIF(TRIM(recipient_province), ''), '" + province.UnknownProvince + "')").
		Scan(&results).Error

	return results, err
}

// queryShopee aggregates shopee_orders (+ their line items) by province.
func (r *CRMRepository) queryShopee(ctx context.Context, query crm.ProvinceQuery) ([]provinceQueryResult, error) {
	db := r.handle(ctx).Model(&shopee.ShopeeOrder{}).
		Joins("LEFT JOIN shopee_order_items ON shopee_order_items.order_id = shopee_orders.id")

	// 1. Date Filter (order_date is a timestamp)
	if from, err := time.Parse("2006-01-02", query.DateFrom); err == nil {
		db = db.Where("shopee_orders.order_date >= ?", from)
	}
	if to, err := time.Parse("2006-01-02", query.DateTo); err == nil {
		db = db.Where("shopee_orders.order_date < ?", to.AddDate(0, 0, 1))
	}

	// 2. Status Filter — imported Shopee orders are COMPLETED; cancelled
	// statuses are kept for forward compatibility.
	switch strings.ToLower(strings.TrimSpace(query.Status)) {
	case "cancelled":
		db = db.Where("shopee_orders.status IN (?)", []string{"CANCELLED", "RETURNED", "REFUNDED"})
	default: // fulfilled/all — Shopee has no UNPAID state
		db = db.Where("shopee_orders.status NOT IN (?)", []string{"CANCELLED", "RETURNED", "REFUNDED"})
	}

	db = applyProvinceFilter(db, "shopee_orders.province", query.Province)

	var results []provinceQueryResult
	err := db.Select(`
		COALESCE(NULLIF(TRIM(shopee_orders.province), ''), '` + province.UnknownProvince + `') AS province,
		COUNT(DISTINCT shopee_orders.id) AS order_count,
		COALESCE(SUM(shopee_order_items.qty), 0) AS item_qty,
		COALESCE(SUM(shopee_order_items.sale_price), 0) AS gross_sales
	`).
		Group("COALESCE(NULLIF(TRIM(shopee_orders.province), ''), '" + province.UnknownProvince + "')").
		Scan(&results).Error

	return results, err
}

func applyProvinceFilter(db *gorm.DB, column, rawProvince string) *gorm.DB {
	clean := strings.TrimSpace(rawProvince)
	if clean == "" || clean == "ทั้งหมด" {
		return db
	}
	if clean == province.UnknownProvince {
		return db.Where("("+column+" IS NULL OR TRIM("+column+") = '' OR "+column+" = ?)", province.UnknownProvince)
	}
	return db.Where(column+" = ?", clean)
}

// mergeProvinceResults sums rows from multiple channels per province and
// sorts by order count, then gross sales (same order the SQL used to emit).
func mergeProvinceResults(rows []provinceQueryResult) []provinceQueryResult {
	if len(rows) == 0 {
		return rows
	}
	byProvince := make(map[string]*provinceQueryResult, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		if existing, ok := byProvince[row.Province]; ok {
			existing.OrderCount += row.OrderCount
			existing.ItemQty += row.ItemQty
			existing.GrossSales += row.GrossSales
		} else {
			cp := row
			byProvince[row.Province] = &cp
			order = append(order, row.Province)
		}
	}
	merged := make([]provinceQueryResult, 0, len(order))
	for _, name := range order {
		merged = append(merged, *byProvince[name])
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].OrderCount != merged[j].OrderCount {
			return merged[i].OrderCount > merged[j].OrderCount
		}
		return merged[i].GrossSales > merged[j].GrossSales
	})
	return merged
}

func roundFloat(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
