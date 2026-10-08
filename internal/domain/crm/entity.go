package crm

// TiktokProvinceSummary represents high-level metrics for the TikTok province report
type TiktokProvinceSummary struct {
	TotalOrders             int64   `json:"totalOrders"`
	TotalItemQty            int64   `json:"totalItemQty"`
	GrossSales              float64 `json:"grossSales"`
	ProvinceCount           int     `json:"provinceCount"`
	TopProvince             string  `json:"topProvince"`
	DataCompletenessPercent float64 `json:"dataCompletenessPercent"`
}

// TiktokProvinceRow represents aggregated statistics for a single province
type TiktokProvinceRow struct {
	Province     string  `json:"province"`
	OrderCount   int64   `json:"orderCount"`
	ItemQty      int64   `json:"itemQty"`
	GrossSales   float64 `json:"grossSales"`
	SharePercent float64 `json:"sharePercent"`
}

// TiktokProvinceReport represents the complete response data
type TiktokProvinceReport struct {
	Summary   TiktokProvinceSummary `json:"summary"`
	Provinces []TiktokProvinceRow   `json:"provinces"`
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
