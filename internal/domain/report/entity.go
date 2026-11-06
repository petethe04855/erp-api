package report

type DashboardSummary struct {
	TotalSales       float64 `json:"total_sales"`
	TotalOrders      int64   `json:"total_orders"`
	PendingOrders    int64   `json:"pending_orders"`
	ShippedOrders    int64   `json:"shipped_orders"`
	TotalSKUs        int64   `json:"total_skus"`
	TotalSuppliers   int64   `json:"total_suppliers"`
	TotalCustomers   int64   `json:"total_customers"`
	TotalUnpaidInvs  int64   `json:"total_unpaid_invoices"`
	UnpaidInvsAmount float64 `json:"unpaid_invoices_amount"`
}

type RevenueRow struct {
	Date      string  `json:"date"`
	Reference string  `json:"reference"`
	Customer  string  `json:"customer"`
	Channel   string  `json:"channel"`
	Amount    float64 `json:"amount"`
}

type RevenueReport struct {
	Rows      []RevenueRow       `json:"rows"`
	Total     float64            `json:"total"`
	ByChannel map[string]float64 `json:"byChannel"`
}

type FinancialSummary struct {
	Revenue           float64 `json:"revenue"`
	COGS              float64 `json:"cogs"`
	GrossProfit       float64 `json:"grossProfit"`
	OperatingExpenses float64 `json:"operatingExpenses"`
	DamageLoss        float64 `json:"damageLoss"`
	NetProfit         float64 `json:"netProfit"`
	GrossMargin       float64 `json:"grossMargin"`
	NetMargin         float64 `json:"netMargin"`
}

type InventoryValuationRow struct {
	SKU          string  `json:"SKU"`
	ProductName  string  `json:"ProductName"`
	Lot          string  `json:"Lot"`
	ExpiryDate   string  `json:"ExpiryDate"`
	RemainingQty int     `json:"RemainingQty"`
	UnitCost     float64 `json:"UnitCost"`
	Value        float64 `json:"Value"`
}

type InventoryValuation struct {
	TotalQty   int                     `json:"totalQty"`
	TotalValue float64                 `json:"totalValue"`
	Rows       []InventoryValuationRow `json:"rows"`
}
