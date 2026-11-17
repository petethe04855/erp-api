package crm

// TiktokProvinceSummary represents high-level metrics for the TikTok province report
type TiktokProvinceSummary struct {
	TotalOrders             int64   `json:"totalOrders"`
	TotalItemQty            int64   `json:"totalItemQty"`
	GrossSales              float64 `json:"grossSales"`
	ProvinceCount           int     `json:"provinceCount"`
	TopProvince             string  `json:"topProvince"`
	DataCompletenessPercent float64 `json:"dataCompletenessPercent"`
	AvgOrderValue           float64 `json:"avgOrderValue"`
	TotalProvinces          int     `json:"totalProvinces"`
	CoveragePercent         float64 `json:"coveragePercent"`
}

// TiktokProvinceRow represents aggregated statistics for a single province
type TiktokProvinceRow struct {
	Province          string  `json:"province"`
	OrderCount        int64   `json:"orderCount"`
	ItemQty           int64   `json:"itemQty"`
	GrossSales        float64 `json:"grossSales"`
	SharePercent      float64 `json:"sharePercent"`
	SalesSharePercent float64 `json:"salesSharePercent"`
	AvgOrderValue     float64 `json:"avgOrderValue"`
	Region            string  `json:"region"`
}

// ProvinceRegionRow represents aggregated statistics for a region of Thailand
type ProvinceRegionRow struct {
	Region        string  `json:"region"`
	ProvinceCount int     `json:"provinceCount"`
	OrderCount    int64   `json:"orderCount"`
	GrossSales    float64 `json:"grossSales"`
	SharePercent  float64 `json:"sharePercent"`
}

// TiktokProvinceReport represents the complete response data
type TiktokProvinceReport struct {
	Summary   TiktokProvinceSummary `json:"summary"`
	Provinces []TiktokProvinceRow   `json:"provinces"`
	Regions   []ProvinceRegionRow   `json:"regions"`
	Trend     SalesTrend            `json:"trend"`
}

// DailyChannelSales is one day of sales for one channel ("tiktok" | "shopee")
type DailyChannelSales struct {
	Day        string  `gorm:"column:day"` // YYYY-MM-DD
	Channel    string  `gorm:"column:channel"`
	OrderCount int64   `gorm:"column:order_count"`
	GrossSales float64 `gorm:"column:gross_sales"`
}

// SalesTrendPoint is sales per channel for one week or month
type SalesTrendPoint struct {
	Period       string  `json:"period"`    // e.g. "2026-W14" or "2026-03"
	StartDate    string  `json:"startDate"` // YYYY-MM-DD
	EndDate      string  `json:"endDate"`   // YYYY-MM-DD
	ShopeeSales  float64 `json:"shopeeSales"`
	TiktokSales  float64 `json:"tiktokSales"`
	TotalSales   float64 `json:"totalSales"`
	ShopeeOrders int64   `json:"shopeeOrders"`
	TiktokOrders int64   `json:"tiktokOrders"`
	TotalOrders  int64   `json:"totalOrders"`
}

// SalesTrend holds weekly (ISO week) and monthly channel trends
type SalesTrend struct {
	Weekly  []SalesTrendPoint `json:"weekly"`
	Monthly []SalesTrendPoint `json:"monthly"`
}

// ProvinceQuery defines query parameters for filtering province reports
type ProvinceQuery struct {
	DateFrom string // Format: YYYY-MM-DD
	DateTo   string // Format: YYYY-MM-DD
	Status   string // "fulfilled" (default), "all", "cancelled"
	Province string // Optional specific province name
	Channel  string // "all" (default), "tiktok", "shopee"
}

type ProvinceSearchRequest struct {
	Province []string `json:"province"`
	Channel  string   `json:"channel"`
	Status   string   `json:"status"`
	DateFrom string   `json:"dateFrom"`
	DateTo   string   `json:"dateTo"`
}
