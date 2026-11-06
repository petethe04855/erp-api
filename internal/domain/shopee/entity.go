package shopee

import "time"

type ShopeeOrder struct {
	ID            string            `json:"id" gorm:"primaryKey;size:64"`
	OrderDate     time.Time         `json:"order_date" gorm:"index;not null"`
	BuyerUsername string            `json:"buyer_username" gorm:"size:255"`
	Province      string            `json:"province" gorm:"size:100"`
	Status        string            `json:"status" gorm:"size:50;not null;default:'COMPLETED'"`
	RowHash       string            `json:"row_hash" gorm:"uniqueIndex;size:64;not null"`
	Items         []ShopeeOrderItem `json:"items" gorm:"foreignKey:OrderID;references:ID;constraint:OnDelete:CASCADE"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func (ShopeeOrder) TableName() string { return "shopee_orders" }

type ShopeeOrderItem struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID      string    `json:"order_id" gorm:"index;not null;size:64"`
	SKU          string    `json:"sku" gorm:"index;not null;size:100"`
	ProductName  string    `json:"product_name" gorm:"size:500;not null"`
	Qty          int       `json:"qty" gorm:"not null;default:1"`
	SalePrice    float64   `json:"sale_price" gorm:"type:numeric(12,2);not null;default:0"` // Line Total (BR-01)
	OriginalSKU  string    `json:"original_sku" gorm:"size:100"`
	SKUConfirmed bool      `json:"sku_confirmed" gorm:"default:false;not null"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (ShopeeOrderItem) TableName() string { return "shopee_order_items" }

type ShopeeIncome struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID      string     `json:"order_id" gorm:"index;not null;size:64"`
	OrderDate    *time.Time `json:"order_date"`
	TransferDate time.Time  `json:"transfer_date" gorm:"index;not null"`
	NetAmount    float64    `json:"net_amount" gorm:"type:numeric(12,2);not null;default:0"`
	RowHash      string     `json:"row_hash" gorm:"uniqueIndex;size:64;not null"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (ShopeeIncome) TableName() string { return "shopee_incomes" }

type MatchingItemRow struct {
	ItemID        uint      `json:"item_id"`
	OrderID       string    `json:"order_id"`
	OrderDate     time.Time `json:"order_date"`
	TransferDate  time.Time `json:"transfer_date"`
	SKU           string    `json:"sku"`
	ProductName   string    `json:"product_name"`
	Qty           int       `json:"qty"`
	EffectiveCost float64   `json:"effective_cost"`
	TotalCost     float64   `json:"total_cost"`
	LineSale      float64   `json:"line_sale"`
	OrderGross    float64   `json:"order_gross"`
	OrderNet      float64   `json:"order_net"`
	AllocatedFee  float64   `json:"allocated_fee"`
	AllocatedNet  float64   `json:"allocated_net"`
	Profit        float64   `json:"profit"`
	MarginPct     float64   `json:"margin_pct"`
	Status        string    `json:"status"` // OK, MISSING_COST, MISSING_ORDER, NEGATIVE_PROFIT
}

type MonthlySummary struct {
	Month           string  `json:"month"`
	OrderCount      int     `json:"order_count"`
	ItemCount       int     `json:"item_count"`
	GrossSale       float64 `json:"gross_sale"`
	PlatformFees    float64 `json:"platform_fees"`
	NetReceive      float64 `json:"net_receive"`
	TotalCost       float64 `json:"total_cost"`
	NetProfit       float64 `json:"net_profit"`
	ProfitMarginPct float64 `json:"profit_margin_pct"`
}

type TopSKUStat struct {
	SKU         string  `json:"sku"`
	ProductName string  `json:"product_name"`
	TotalQty    int     `json:"total_qty"`
	TotalGross  float64 `json:"total_gross"`
	TotalCost   float64 `json:"total_cost"`
	NetProfit   float64 `json:"net_profit"`
	MarginPct   float64 `json:"margin_pct"`
}
