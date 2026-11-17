package crm

import (
	"fmt"
	"time"

	"chawy-erp-api/internal/domain/crm"
)

const dayLayout = "2006-01-02"

// buildSalesTrend groups daily channel sales into ISO weeks and calendar
// months covering [from, to]. Periods without sales are included with zeros
// so charts show a continuous timeline.
func buildSalesTrend(daily []crm.DailyChannelSales, from, to time.Time) crm.SalesTrend {
	weekly := newBuckets()
	monthly := newBuckets()

	// Seed every period in range.
	for d := weekStart(from); !d.After(to); d = d.AddDate(0, 0, 7) {
		weekly.ensure(weekKey(d), d, d.AddDate(0, 0, 6))
	}
	for d := monthStart(from); !d.After(to); d = d.AddDate(0, 1, 0) {
		monthly.ensure(d.Format("2006-01"), d, d.AddDate(0, 1, -1))
	}

	for _, row := range daily {
		day, err := time.ParseInLocation(dayLayout, row.Day, from.Location())
		if err != nil {
			continue
		}
		ws := weekStart(day)
		ms := monthStart(day)
		weekly.ensure(weekKey(ws), ws, ws.AddDate(0, 0, 6)).add(row)
		monthly.ensure(ms.Format("2006-01"), ms, ms.AddDate(0, 1, -1)).add(row)
	}

	return crm.SalesTrend{Weekly: weekly.list(), Monthly: monthly.list()}
}

type buckets struct {
	order []string
	byKey map[string]*crm.SalesTrendPoint
}

func newBuckets() *buckets {
	return &buckets{byKey: map[string]*crm.SalesTrendPoint{}}
}

func (b *buckets) ensure(key string, start, end time.Time) *trendPoint {
	if p, ok := b.byKey[key]; ok {
		return (*trendPoint)(p)
	}
	p := &crm.SalesTrendPoint{Period: key, StartDate: start.Format(dayLayout), EndDate: end.Format(dayLayout)}
	b.byKey[key] = p
	b.order = append(b.order, key)
	return (*trendPoint)(p)
}

func (b *buckets) list() []crm.SalesTrendPoint {
	out := make([]crm.SalesTrendPoint, 0, len(b.order))
	for _, k := range b.order {
		p := b.byKey[k]
		p.ShopeeSales = round2(p.ShopeeSales)
		p.TiktokSales = round2(p.TiktokSales)
		p.TotalSales = round2(p.ShopeeSales + p.TiktokSales)
		p.TotalOrders = p.ShopeeOrders + p.TiktokOrders
		out = append(out, *p)
	}
	// Seeded periods are chronological; rows outside the range (should not
	// happen) are appended after, so sort by start date to be safe.
	sortByStart(out)
	return out
}

type trendPoint crm.SalesTrendPoint

func (p *trendPoint) add(row crm.DailyChannelSales) {
	switch row.Channel {
	case "shopee":
		p.ShopeeSales += row.GrossSales
		p.ShopeeOrders += row.OrderCount
	default:
		p.TiktokSales += row.GrossSales
		p.TiktokOrders += row.OrderCount
	}
}

func sortByStart(points []crm.SalesTrendPoint) {
	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j].StartDate < points[j-1].StartDate; j-- {
			points[j], points[j-1] = points[j-1], points[j]
		}
	}
}

// weekStart returns the Monday of the ISO week containing t.
func weekStart(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	offset := (int(t.Weekday()) + 6) % 7 // Monday = 0
	return t.AddDate(0, 0, -offset)
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func weekKey(monday time.Time) string {
	y, w := monday.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}
