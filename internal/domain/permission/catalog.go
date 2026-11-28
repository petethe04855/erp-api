package permission

// Page is one screen of erp-web-v2 whose access the owner can configure.
// Pages that only the owner may open (users, settings, TikTok setup and the
// role permissions page itself) are deliberately not in the catalog.
type Page struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Group string `json:"group"`
	// Path is the web route the page lives at (used by the frontend guard).
	Path string `json:"path"`
	// Actions are the rights that mean something on this page. On orders
	// and returns the "delete" right is what cancels a document.
	Actions []Action `json:"actions"`
	// Defaults is the access each role has until the owner saves the matrix.
	// They mirror the RequireRole rules and sidebar roles that existed before
	// the matrix, so turning the feature on does not change who can do what.
	Defaults map[string]Access `json:"-"`
}

// Supports reports whether the page has the given action.
func (p Page) Supports(action Action) bool {
	for _, a := range p.Actions {
		if a == action {
			return true
		}
	}
	return false
}

// DefaultFor returns the default access of role on the page.
func (p Page) DefaultFor(role string) Access {
	return p.Defaults[role].Limit(p)
}

var (
	viewOnly = []Action{View}
	crud     = []Action{View, Create, Edit, Delete}
	cru      = []Action{View, Create, Edit}
	cr       = []Action{View, Create}
)

// grants builds a Defaults map from the roles holding each action.
func grants(view, create, edit, del []string) map[string]Access {
	out := map[string]Access{}
	set := func(roles []string, apply func(*Access)) {
		for _, r := range roles {
			a := out[r]
			apply(&a)
			out[r] = a
		}
	}
	set(view, func(a *Access) { a.View = true })
	set(create, func(a *Access) { a.Create = true })
	set(edit, func(a *Access) { a.Edit = true })
	set(del, func(a *Access) { a.Delete = true })
	return out
}

func roles(r ...string) []string { return r }

var allStaff = roles("sales", "warehouse", "accountant", "live")

// Page keys referenced by the API routes.
const (
	PageDashboard        = "dashboard"
	PageCRMProvinces     = "crm_provinces"
	PageSKU              = "sku"
	PageInventory        = "inventory"
	PageGoodsReceive     = "goods_receive"
	PageGoodsIssue       = "goods_issue"
	PageOrders           = "orders"
	PageQuotation        = "quotation"
	PageInvoices         = "invoices"
	PageReturns          = "returns"
	PageCustomers        = "customers"
	PagePurchaseOrder    = "purchase_order"
	PageBooth            = "booth"
	PageStockOut         = "stock_out"
	PageMyLive           = "my_live"
	PageTikTokOrders     = "tiktok_orders"
	PageTikTokLive       = "tiktok_live"
	PageTikTokCalculator = "tiktok_calculator"
	PageShopeeInsights   = "shopee_insights"
	PageShopeeOrders     = "shopee_orders"
	PageShopeeIncome     = "shopee_income"
	PageShopeeMatching   = "shopee_matching"
	PageFinanceJournal   = "finance_journal"
	PageFinanceExpenses  = "finance_expenses"
	PageFinanceReports   = "finance_reports"
	PageReports          = "reports"
)

// Catalog is the ordered list of configurable pages.
var Catalog = []Page{
	{Key: PageDashboard, Label: "Dashboard", Group: "ภาพรวม", Path: "/dashboard", Actions: viewOnly,
		Defaults: grants(allStaff, nil, nil, nil)},
	{Key: PageCRMProvinces, Label: "ลูกค้าตามจังหวัด (CRM)", Group: "ภาพรวม", Path: "/crm/tiktok-provinces", Actions: viewOnly,
		Defaults: grants(roles("sales"), nil, nil, nil)},

	{Key: PageSKU, Label: "SKU Master", Group: "สินค้าและคลัง", Path: "/sku", Actions: crud,
		Defaults: grants(allStaff, roles("warehouse", "sales"), roles("warehouse", "sales"), roles("accountant"))},
	{Key: PageInventory, Label: "สต็อกคงคลัง (Inventory)", Group: "สินค้าและคลัง", Path: "/inventory", Actions: crud,
		Defaults: grants(allStaff, roles("warehouse"), roles("warehouse"), roles("warehouse"))},
	{Key: PageGoodsReceive, Label: "รับสินค้า (Goods Receive)", Group: "สินค้าและคลัง", Path: "/goods-receive", Actions: cr,
		Defaults: grants(roles("warehouse"), roles("warehouse"), nil, nil)},
	{Key: PageGoodsIssue, Label: "เบิกสินค้า (Goods Issue)", Group: "สินค้าและคลัง", Path: "/goods-issue", Actions: cr,
		Defaults: grants(roles("warehouse"), roles("warehouse"), nil, nil)},
	{Key: PageStockOut, Label: "นับของออกรายวัน (Stock-out)", Group: "สินค้าและคลัง", Path: "/stock-out", Actions: cru,
		Defaults: grants(roles("warehouse", "sales", "accountant"), roles("warehouse", "sales", "accountant"), roles("warehouse", "sales", "accountant"), nil)},

	{Key: PageOrders, Label: "คำสั่งซื้อ (Orders)", Group: "การขายและลูกค้า", Path: "/orders", Actions: crud,
		Defaults: grants(roles("sales", "accountant"), roles("sales"), roles("sales", "warehouse"), roles("sales"))},
	{Key: PageQuotation, Label: "ใบเสนอราคา (Quotation)", Group: "การขายและลูกค้า", Path: "/quotation", Actions: cru,
		Defaults: grants(roles("sales"), roles("sales"), roles("sales"), nil)},
	{Key: PageInvoices, Label: "ใบแจ้งหนี้ (Invoices)", Group: "การขายและลูกค้า", Path: "/invoices", Actions: cru,
		Defaults: grants(roles("accountant", "sales"), roles("accountant", "sales"), roles("accountant"), nil)},
	{Key: PageReturns, Label: "รับคืนสินค้า (Returns)", Group: "การขายและลูกค้า", Path: "/returns", Actions: crud,
		Defaults: grants(roles("sales", "accountant", "warehouse"), roles("sales"), roles("sales", "warehouse"), roles("sales"))},
	{Key: PageCustomers, Label: "ลูกค้า (Customers)", Group: "การขายและลูกค้า", Path: "/customers", Actions: crud,
		Defaults: grants(roles("sales"), roles("sales"), roles("sales"), roles("accountant"))},
	{Key: PageBooth, Label: "ขายหน้าบูธ (Booth)", Group: "การขายและลูกค้า", Path: "/booth", Actions: cru,
		Defaults: grants(roles("sales"), roles("sales"), roles("sales"), nil)},

	{Key: PagePurchaseOrder, Label: "สั่งซื้อ (Purchase Order)", Group: "การจัดซื้อ", Path: "/purchase-order", Actions: cru,
		Defaults: grants(roles("warehouse", "accountant"), roles("warehouse", "accountant"), roles("warehouse", "accountant"), nil)},

	{Key: PageMyLive, Label: "บันทึกและรายได้ของฉัน", Group: "TikTok Shop", Path: "/my-live", Actions: cru,
		Defaults: grants(roles("live"), roles("live"), roles("live"), nil)},
	{Key: PageTikTokOrders, Label: "TikTok Orders", Group: "TikTok Shop", Path: "/tiktok-orders", Actions: cru,
		Defaults: grants(roles("sales", "warehouse"), roles("warehouse"), roles("sales", "warehouse"), nil)},
	{Key: PageTikTokLive, Label: "Live & Content", Group: "TikTok Shop", Path: "/tiktok-live", Actions: crud,
		Defaults: grants(roles("sales", "warehouse"), roles("sales", "warehouse"), roles("sales"), roles("sales"))},
	{Key: PageTikTokCalculator, Label: "Fee Calculator", Group: "TikTok Shop", Path: "/tiktok-calculator", Actions: viewOnly,
		Defaults: grants(roles("sales", "accountant"), nil, nil, nil)},

	{Key: PageShopeeInsights, Label: "Shopee เชิงลึก", Group: "Shopee", Path: "/shopee/insights", Actions: viewOnly,
		Defaults: grants(roles("accountant", "sales"), nil, nil, nil)},
	{Key: PageShopeeOrders, Label: "Shopee Orders", Group: "Shopee", Path: "/shopee/orders", Actions: crud,
		Defaults: grants(roles("sales", "warehouse", "accountant"), roles("sales", "warehouse", "accountant"), roles("sales", "warehouse", "accountant"), roles("accountant"))},
	{Key: PageShopeeIncome, Label: "Shopee Income", Group: "Shopee", Path: "/shopee/income", Actions: []Action{View, Create, Delete},
		Defaults: grants(roles("accountant"), roles("sales", "warehouse", "accountant"), nil, roles("accountant"))},
	{Key: PageShopeeMatching, Label: "Payout Matching", Group: "Shopee", Path: "/shopee/matching", Actions: viewOnly,
		Defaults: grants(roles("accountant", "sales"), nil, nil, nil)},

	{Key: PageFinanceJournal, Label: "สมุดรายวัน (Journal)", Group: "การเงินและบัญชี", Path: "/finance/journal", Actions: viewOnly,
		Defaults: grants(roles("accountant"), nil, nil, nil)},
	{Key: PageFinanceExpenses, Label: "ค่าใช้จ่าย (Expenses)", Group: "การเงินและบัญชี", Path: "/finance/expenses", Actions: crud,
		Defaults: grants(roles("accountant"), roles("accountant"), roles("accountant"), roles("accountant"))},
	{Key: PageFinanceReports, Label: "รายงานการเงิน (Reports)", Group: "การเงินและบัญชี", Path: "/finance/reports", Actions: viewOnly,
		Defaults: grants(roles("accountant"), nil, nil, nil)},

	{Key: PageReports, Label: "รายงานเดิม (Legacy Reports)", Group: "รายงานและระบบ", Path: "/reports", Actions: viewOnly,
		Defaults: grants(roles("accountant"), nil, nil, nil)},
}

// FindPage returns the catalog entry with key.
func FindPage(key string) (Page, bool) {
	for _, p := range Catalog {
		if p.Key == key {
			return p, true
		}
	}
	return Page{}, false
}
