package shopee

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	domainShopee "chawy-erp-api/internal/domain/shopee"
)

// Shopee sales insights (plan/SHOPEE_SALES_INSIGHTS.md): sales by order date,
// products, set sizes and repeat-purchase behaviour of the Chawy shop. Every
// endpoint returns totals only — never a buyer username or address.

var ErrInvalidInsightRange = errors.New("invalid range: use from/to as YYYY-MM, to >= from, at most 24 months")

const maxInsightMonths = 24

// InsightRange is a span of whole months in Bangkok time.
type InsightRange struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Months []string `json:"-"`
	Start  time.Time
	End    time.Time // exclusive
}

// ParseInsightRange reads `from`/`to` (YYYY-MM). Empty values default to
// January of this year through the current month.
func ParseInsightRange(from, to string, now time.Time) (InsightRange, error) {
	now = now.In(shopeeTZ)
	if from == "" {
		from = fmt.Sprintf("%04d-01", now.Year())
	}
	if to == "" {
		to = now.Format("2006-01")
	}
	start, err1 := time.ParseInLocation("2006-01", from, shopeeTZ)
	last, err2 := time.ParseInLocation("2006-01", to, shopeeTZ)
	if err1 != nil || err2 != nil || last.Before(start) {
		return InsightRange{}, ErrInvalidInsightRange
	}
	var months []string
	for m := start; !m.After(last); m = m.AddDate(0, 1, 0) {
		months = append(months, m.Format("2006-01"))
	}
	if len(months) > maxInsightMonths {
		return InsightRange{}, ErrInvalidInsightRange
	}
	return InsightRange{From: from, To: to, Months: months, Start: start, End: last.AddDate(0, 1, 0)}, nil
}

// InsightMeta is attached to every insight response.
type InsightMeta struct {
	From           string    `json:"from"`
	To             string    `json:"to"`
	OrdersScanned  int       `json:"orders_scanned"`
	UnnamedOrders  int       `json:"unnamed_orders"`
	ExcludedOrders int       `json:"excluded_orders"` // sister-shop orders left out
	GeneratedAt    time.Time `json:"generated_at"`
}

// ---- response types ----

type InsightKPI struct {
	TotalSales     float64  `json:"total_sales"`
	Orders         int      `json:"orders"`
	Customers      int      `json:"customers"`
	AOV            float64  `json:"aov"`
	RepeatCust     int      `json:"repeat_customers"`
	RepeatPct      float64  `json:"repeat_pct"`
	RepeatSalesPct float64  `json:"repeat_sales_pct"`
	MedianGapDays  *float64 `json:"median_gap_days"`
	LastMonth      string   `json:"last_month"`
	PrevMonth      string   `json:"prev_month"`
	LastSales      float64  `json:"last_sales"`
	PrevSales      float64  `json:"prev_sales"`
	MoMPct         *float64 `json:"mom_pct"`
}

type InsightMonth struct {
	Month        string  `json:"month"`
	Sales        float64 `json:"sales"`
	Orders       int     `json:"orders"`
	NewCustomers int     `json:"new_customers"`
	RepeatOrders int     `json:"repeat_orders"`
}

type InsightSummary struct {
	Meta    InsightMeta    `json:"meta"`
	KPI     InsightKPI     `json:"kpi"`
	Monthly []InsightMonth `json:"monthly"`
}

type InsightMonthCategory struct {
	Month    string  `json:"month"`
	Category string  `json:"category"`
	Sales    float64 `json:"sales"`
}

type InsightProduct struct {
	Category string  `json:"category"`
	Product  string  `json:"product"`
	SKU      string  `json:"sku"`
	Orders   int     `json:"orders"`
	Qty      int     `json:"qty"`
	Sales    float64 `json:"sales"`
}

type InsightProducts struct {
	Meta            InsightMeta            `json:"meta"`
	Categories      []string               `json:"categories"`
	MonthlyCategory []InsightMonthCategory `json:"monthly_category"`
	Products        []InsightProduct       `json:"products"`
}

type InsightMonthSet struct {
	Month    string  `json:"month"`
	Set      int     `json:"set"`
	SetsSold int     `json:"sets_sold"`
	Sales    float64 `json:"sales"`
}

type InsightBagsBucket struct {
	Bags   string  `json:"bags"`
	Orders int     `json:"orders"`
	Sales  float64 `json:"sales"`
}

type InsightSets struct {
	Meta            InsightMeta         `json:"meta"`
	SetSizes        []int               `json:"set_sizes"`
	MonthlySets     []InsightMonthSet   `json:"monthly_sets"`
	BagsPerOrder    []InsightBagsBucket `json:"bags_per_order"`
	UnresolvedLines int                 `json:"unresolved_lines"`
}

type InsightFrequency struct {
	Purchases int     `json:"purchases"`
	Customers int     `json:"customers"`
	Sales     float64 `json:"sales"`
}

type InsightGapWeek struct {
	Week      string  `json:"week"`
	Customers int     `json:"customers"`
	CumPct    float64 `json:"cum_pct"`
}

type InsightRepeatGap struct {
	MedianDays *float64         `json:"median_days"`
	Weeks      []InsightGapWeek `json:"weeks"`
}

type InsightCustomers struct {
	Meta      InsightMeta        `json:"meta"`
	Frequency []InsightFrequency `json:"frequency"`
	RepeatGap InsightRepeatGap   `json:"repeat_gap"`
}

type InsightJourneyLink struct {
	FromOrder int    `json:"from_order"`
	From      string `json:"from"`
	ToOrder   int    `json:"to_order"`
	To        string `json:"to"`
	Customers int    `json:"customers"`
}

type InsightJourney struct {
	Meta  InsightMeta          `json:"meta"`
	Steps int                  `json:"steps"`
	Nodes []string             `json:"nodes"`
	Links []InsightJourneyLink `json:"links"`
}

// InsightUsecase computes the five insight endpoints.
type InsightUsecase interface {
	Summary(ctx context.Context, rng InsightRange) (*InsightSummary, error)
	Products(ctx context.Context, rng InsightRange) (*InsightProducts, error)
	Sets(ctx context.Context, rng InsightRange) (*InsightSets, error)
	Customers(ctx context.Context, rng InsightRange) (*InsightCustomers, error)
	Journey(ctx context.Context, rng InsightRange, steps int) (*InsightJourney, error)
}

type insightUsecase struct {
	repo domainShopee.InsightRepository
	now  func() time.Time
}

func NewInsightUsecase(repo domainShopee.InsightRepository) InsightUsecase {
	return &insightUsecase{repo: repo, now: time.Now}
}

// ---- shared dataset ----

type iLine struct {
	product, sku string
	qty          int
	sales        float64
	paid         bool // freebies (sale price 0) count for the order, not for sales or units
	category     string
	fd           bool
	set          int
	setResolved  bool
	rod          bool
}

type iOrder struct {
	id    string
	date  time.Time
	month string
	buyer string
	lines []iLine
	sales float64
}

type dataset struct {
	rng     InsightRange
	orders  []*iOrder // in range, oldest first
	all     []*iOrder // whole history up to rng.End (sister shop excluded)
	firstID map[string]string
	meta    InsightMeta
}

func (u *insightUsecase) load(ctx context.Context, rng InsightRange) (*dataset, error) {
	raw, err := u.repo.ListInsightLines(ctx, rng.End)
	if err != nil {
		return nil, err
	}
	byID := map[string]*iOrder{}
	excluded := map[string]bool{}
	var list []*iOrder
	for _, l := range raw {
		if IsOtherShopProduct(l.ProductName) {
			excluded[l.OrderID] = true
		}
		o := byID[l.OrderID]
		if o == nil {
			d := l.OrderDate.In(shopeeTZ)
			o = &iOrder{id: l.OrderID, date: d, month: d.Format("2006-01"), buyer: l.BuyerUsername}
			byID[l.OrderID] = o
			list = append(list, o)
		}
		paid := l.SalePrice > 0
		ln := iLine{product: l.ProductName, sku: l.SKU, qty: l.Qty, sales: l.SalePrice, paid: paid,
			category: ProductCategory(l.ProductName), fd: IsFreezeDried(l.ProductName), rod: fishingRodRe.MatchString(l.ProductName)}
		if ln.fd {
			ln.set, ln.setResolved = SetSize(l.ProductName, l.VariationName, l.SKU)
		}
		o.lines = append(o.lines, ln)
		if paid {
			o.sales += l.SalePrice
		}
	}

	ds := &dataset{rng: rng, firstID: map[string]string{}}
	ds.meta = InsightMeta{From: rng.From, To: rng.To, GeneratedAt: u.now().In(shopeeTZ)}
	sort.SliceStable(list, func(i, j int) bool { return list[i].date.Before(list[j].date) })
	for _, o := range list {
		inRange := !o.date.Before(rng.Start)
		if excluded[o.id] {
			if inRange {
				ds.meta.ExcludedOrders++
			}
			continue
		}
		ds.all = append(ds.all, o)
		if o.buyer != "" {
			if _, seen := ds.firstID[o.buyer]; !seen {
				ds.firstID[o.buyer] = o.id
			}
		}
		if inRange {
			ds.orders = append(ds.orders, o)
			if o.buyer == "" {
				ds.meta.UnnamedOrders++
			}
		}
	}
	ds.meta.OrdersScanned = len(ds.orders)
	return ds, nil
}

// byBuyer groups the in-range orders of named buyers, oldest first.
func (ds *dataset) byBuyer() map[string][]*iOrder {
	m := map[string][]*iOrder{}
	for _, o := range ds.orders {
		if o.buyer != "" {
			m[o.buyer] = append(m[o.buyer], o)
		}
	}
	return m
}

// repeatGaps are the days from each repeat buyer's first to second order in range.
func (ds *dataset) repeatGaps() []int {
	var gaps []int
	for _, os := range ds.byBuyer() {
		if len(os) >= 2 {
			gaps = append(gaps, int(os[1].date.Sub(os[0].date).Hours()/24))
		}
	}
	sort.Ints(gaps)
	return gaps
}

func median(v []int) *float64 {
	if len(v) == 0 {
		return nil
	}
	var m float64
	if n := len(v); n%2 == 1 {
		m = float64(v[n/2])
	} else {
		m = float64(v[n/2-1]+v[n/2]) / 2
	}
	return &m
}

func pct(part, whole float64) float64 {
	if whole == 0 {
		return 0
	}
	return round2(part / whole * 100)
}

// mainProduct labels an order by its biggest category: freeze-dried (named by
// the set size of its biggest freeze-dried line), the fishing rod, or the rest.
func mainProduct(o *iOrder) string {
	var fd, rod, other float64
	bestFD, bestSet := -1.0, 1
	for _, l := range o.lines {
		switch {
		case l.fd:
			fd += l.sales
			if l.sales > bestFD {
				bestFD, bestSet = l.sales, l.set
			}
		case l.rod:
			rod += l.sales
		default:
			other += l.sales
		}
	}
	switch {
	case fd > 0 && fd >= rod && fd >= other:
		return BagsLabel(bestSet)
	case rod > 0 && rod >= other:
		return MainFishingRod
	case fd == 0 && rod == 0 && other == 0 && bestFD >= 0:
		return BagsLabel(bestSet) // freebie-only order of a freeze-dried line
	default:
		return MainToyOrOther
	}
}

// ---- endpoints ----

func (u *insightUsecase) Summary(ctx context.Context, rng InsightRange) (*InsightSummary, error) {
	ds, err := u.load(ctx, rng)
	if err != nil {
		return nil, err
	}
	idx := map[string]int{}
	monthly := make([]InsightMonth, len(rng.Months))
	for i, m := range rng.Months {
		monthly[i] = InsightMonth{Month: m}
		idx[m] = i
	}

	kpi := InsightKPI{}
	customers := map[string]float64{}
	orderCount := map[string]int{}
	for _, o := range ds.orders {
		kpi.TotalSales += o.sales
		kpi.Orders++
		mm := &monthly[idx[o.month]]
		mm.Sales += o.sales
		mm.Orders++
		if o.buyer == "" {
			continue
		}
		customers[o.buyer] += o.sales
		orderCount[o.buyer]++
		if ds.firstID[o.buyer] == o.id {
			mm.NewCustomers++
		} else {
			mm.RepeatOrders++
		}
	}
	kpi.Customers = len(customers)
	var repeatSales float64
	for b, n := range orderCount {
		if n >= 2 {
			kpi.RepeatCust++
			repeatSales += customers[b]
		}
	}
	if kpi.Orders > 0 {
		kpi.AOV = round2(kpi.TotalSales / float64(kpi.Orders))
	}
	kpi.RepeatPct = pct(float64(kpi.RepeatCust), float64(kpi.Customers))
	kpi.RepeatSalesPct = pct(repeatSales, kpi.TotalSales)
	kpi.MedianGapDays = median(ds.repeatGaps())
	kpi.TotalSales = round2(kpi.TotalSales)

	// Month-over-month: the last month in range with sales against the month
	// before it (taken from the whole history, so a one-month range works too).
	for i := len(monthly) - 1; i >= 0; i-- {
		if monthly[i].Orders > 0 {
			kpi.LastMonth = monthly[i].Month
			kpi.LastSales = round2(monthly[i].Sales)
			break
		}
	}
	if kpi.LastMonth != "" {
		t, _ := time.ParseInLocation("2006-01", kpi.LastMonth, shopeeTZ)
		kpi.PrevMonth = t.AddDate(0, -1, 0).Format("2006-01")
		for _, o := range ds.all {
			if o.month == kpi.PrevMonth {
				kpi.PrevSales += o.sales
			}
		}
		kpi.PrevSales = round2(kpi.PrevSales)
		if kpi.PrevSales > 0 {
			v := round2((kpi.LastSales - kpi.PrevSales) / kpi.PrevSales * 100)
			kpi.MoMPct = &v
		}
	}
	for i := range monthly {
		monthly[i].Sales = round2(monthly[i].Sales)
	}
	return &InsightSummary{Meta: ds.meta, KPI: kpi, Monthly: monthly}, nil
}

func (u *insightUsecase) Products(ctx context.Context, rng InsightRange) (*InsightProducts, error) {
	ds, err := u.load(ctx, rng)
	if err != nil {
		return nil, err
	}
	catSales := map[string]map[string]float64{}
	type agg struct {
		p      InsightProduct
		orders map[string]bool
		skus   map[string]int
	}
	prods := map[string]*agg{}
	for _, o := range ds.orders {
		for _, l := range o.lines {
			if !l.paid {
				continue
			}
			if catSales[o.month] == nil {
				catSales[o.month] = map[string]float64{}
			}
			catSales[o.month][l.category] += l.sales
			a := prods[l.product]
			if a == nil {
				a = &agg{p: InsightProduct{Category: l.category, Product: l.product}, orders: map[string]bool{}, skus: map[string]int{}}
				prods[l.product] = a
			}
			a.orders[o.id] = true
			a.p.Qty += l.qty
			a.p.Sales += l.sales
			if l.sku != "" {
				a.skus[l.sku]++
			}
		}
	}

	res := &InsightProducts{Meta: ds.meta, Categories: InsightCategories}
	for _, m := range rng.Months {
		for _, c := range InsightCategories {
			res.MonthlyCategory = append(res.MonthlyCategory, InsightMonthCategory{Month: m, Category: c, Sales: round2(catSales[m][c])})
		}
	}
	for _, a := range prods {
		a.p.Orders = len(a.orders)
		a.p.Sales = round2(a.p.Sales)
		best := 0
		for sku, n := range a.skus {
			if n > best || (n == best && sku < a.p.SKU) {
				a.p.SKU, best = sku, n
			}
		}
		res.Products = append(res.Products, a.p)
	}
	sort.Slice(res.Products, func(i, j int) bool {
		a, b := res.Products[i], res.Products[j]
		if a.Sales != b.Sales {
			return a.Sales > b.Sales
		}
		return a.Product < b.Product
	})
	return res, nil
}

var bagBuckets = []string{"1", "2", "3", "4", "5", "6–9", "10", "11+"}

func bagBucket(n int) string {
	switch {
	case n <= 5:
		return strconv.Itoa(n)
	case n <= 9:
		return "6–9"
	case n == 10:
		return "10"
	default:
		return "11+"
	}
}

func (u *insightUsecase) Sets(ctx context.Context, rng InsightRange) (*InsightSets, error) {
	ds, err := u.load(ctx, rng)
	if err != nil {
		return nil, err
	}
	type key struct {
		month string
		set   int
	}
	sets := map[key]*InsightMonthSet{}
	sizes := map[int]bool{}
	buckets := map[string]*InsightBagsBucket{}
	res := &InsightSets{Meta: ds.meta}
	for _, o := range ds.orders {
		bags := 0
		for _, l := range o.lines {
			if !l.fd || !l.paid {
				continue
			}
			if !l.setResolved {
				res.UnresolvedLines++
			}
			sizes[l.set] = true
			k := key{o.month, l.set}
			if sets[k] == nil {
				sets[k] = &InsightMonthSet{Month: o.month, Set: l.set}
			}
			sets[k].SetsSold += l.qty
			sets[k].Sales += l.sales
			bags += l.qty * l.set
		}
		if bags > 0 {
			b := bagBucket(bags)
			if buckets[b] == nil {
				buckets[b] = &InsightBagsBucket{Bags: b}
			}
			buckets[b].Orders++
			buckets[b].Sales += o.sales
		}
	}
	for s := range sizes {
		res.SetSizes = append(res.SetSizes, s)
	}
	sort.Ints(res.SetSizes)
	for _, m := range rng.Months {
		for _, s := range res.SetSizes {
			row := InsightMonthSet{Month: m, Set: s}
			if v := sets[key{m, s}]; v != nil {
				row = *v
			}
			row.Sales = round2(row.Sales)
			res.MonthlySets = append(res.MonthlySets, row)
		}
	}
	for _, b := range bagBuckets {
		row := InsightBagsBucket{Bags: b}
		if v := buckets[b]; v != nil {
			row = *v
		}
		row.Sales = round2(row.Sales)
		res.BagsPerOrder = append(res.BagsPerOrder, row)
	}
	return res, nil
}

const gapWeeks = 20

func (u *insightUsecase) Customers(ctx context.Context, rng InsightRange) (*InsightCustomers, error) {
	ds, err := u.load(ctx, rng)
	if err != nil {
		return nil, err
	}
	res := &InsightCustomers{Meta: ds.meta}
	counts := map[int]*InsightFrequency{}
	maxN := 0
	for _, os := range ds.byBuyer() {
		n := len(os)
		if counts[n] == nil {
			counts[n] = &InsightFrequency{Purchases: n}
		}
		counts[n].Customers++
		for _, o := range os {
			counts[n].Sales += o.sales
		}
		if n > maxN {
			maxN = n
		}
	}
	for n := 1; n <= maxN; n++ {
		row := InsightFrequency{Purchases: n}
		if v := counts[n]; v != nil {
			row = *v
		}
		row.Sales = round2(row.Sales)
		res.Frequency = append(res.Frequency, row)
	}

	gaps := ds.repeatGaps()
	res.RepeatGap.MedianDays = median(gaps)
	weekCounts := make([]int, gapWeeks+1)
	for _, d := range gaps {
		w := d / 7
		if w > gapWeeks {
			w = gapWeeks
		}
		weekCounts[w]++
	}
	cum := 0
	for i, c := range weekCounts {
		label := strconv.Itoa(i + 1)
		if i == gapWeeks {
			label = strconv.Itoa(gapWeeks+1) + "+"
		}
		cum += c
		res.RepeatGap.Weeks = append(res.RepeatGap.Weeks, InsightGapWeek{Week: label, Customers: c, CumPct: pct(float64(cum), float64(len(gaps)))})
	}
	return res, nil
}

func (u *insightUsecase) Journey(ctx context.Context, rng InsightRange, steps int) (*InsightJourney, error) {
	if steps < 2 || steps > 6 {
		return nil, fmt.Errorf("steps must be between 2 and 6")
	}
	ds, err := u.load(ctx, rng)
	if err != nil {
		return nil, err
	}
	type key struct {
		from     int
		fromNode string
		toNode   string
	}
	links := map[key]int{}
	nodes := map[string]bool{}
	for _, os := range ds.byBuyer() {
		n := len(os)
		if n > steps {
			n = steps
		}
		labels := make([]string, n)
		for i := 0; i < n; i++ {
			labels[i] = mainProduct(os[i])
			nodes[labels[i]] = true
		}
		for i := 0; i < n-1; i++ {
			links[key{i + 1, labels[i], labels[i+1]}]++
		}
		if n < steps {
			links[key{n, labels[n-1], JourneyStopNode}]++
			nodes[JourneyStopNode] = true
		}
	}

	res := &InsightJourney{Meta: ds.meta, Steps: steps}
	for n := range nodes {
		res.Nodes = append(res.Nodes, n)
	}
	sort.Slice(res.Nodes, func(i, j int) bool { return journeyNodeRank(res.Nodes[i]) < journeyNodeRank(res.Nodes[j]) })
	for k, c := range links {
		res.Links = append(res.Links, InsightJourneyLink{FromOrder: k.from, From: k.fromNode, ToOrder: k.from + 1, To: k.toNode, Customers: c})
	}
	sort.Slice(res.Links, func(i, j int) bool {
		a, b := res.Links[i], res.Links[j]
		if a.FromOrder != b.FromOrder {
			return a.FromOrder < b.FromOrder
		}
		if a.From != b.From {
			return journeyNodeRank(a.From) < journeyNodeRank(b.From)
		}
		return journeyNodeRank(a.To) < journeyNodeRank(b.To)
	})
	return res, nil
}

// journeyNodeRank orders nodes: freeze-dried by set size, the rod, the rest, then "not yet".
func journeyNodeRank(n string) int {
	switch n {
	case MainFishingRod:
		return 1000
	case MainToyOrOther:
		return 1001
	case JourneyStopNode:
		return 1002
	}
	var size int
	if _, err := fmt.Sscanf(n, "ฟรีซดราย %d ถุง", &size); err == nil {
		return size
	}
	return 999
}
