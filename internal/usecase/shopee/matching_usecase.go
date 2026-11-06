package shopee

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"sort"
	"strconv"

	domainShopee "chawy-erp-api/internal/domain/shopee"
	usecaseSKU "chawy-erp-api/internal/usecase/sku"
)

type MatchingUsecase interface {
	GetMonthlyMatching(ctx context.Context, year int, month int) ([]domainShopee.MatchingItemRow, *domainShopee.MonthlySummary, error)
	ExportMonthlyMatchingCSV(ctx context.Context, year int, month int) ([]byte, string, error)
}

type matchingUsecase struct {
	incomeRepo  domainShopee.IncomeRepository
	orderRepo   domainShopee.OrderRepository
	costUsecase usecaseSKU.CostHistoryUsecase
}

func NewMatchingUsecase(
	incomeRepo domainShopee.IncomeRepository,
	orderRepo domainShopee.OrderRepository,
	costUsecase usecaseSKU.CostHistoryUsecase,
) MatchingUsecase {
	return &matchingUsecase{
		incomeRepo:  incomeRepo,
		orderRepo:   orderRepo,
		costUsecase: costUsecase,
	}
}

func round2(val float64) float64 {
	return math.Round(val*100) / 100
}

func (u *matchingUsecase) calculateMonth(ctx context.Context, year int, month int) ([]domainShopee.MatchingItemRow, *domainShopee.MonthlySummary, error) {
	incomes, err := u.incomeRepo.FindByMonth(ctx, year, month)
	if err != nil {
		return nil, nil, err
	}

	monthStr := fmt.Sprintf("%04d-%02d", year, month)
	summary := &domainShopee.MonthlySummary{
		Month: monthStr,
	}

	if len(incomes) == 0 {
		return []domainShopee.MatchingItemRow{}, summary, nil
	}

	orderIDs := make([]string, len(incomes))
	incomeMap := make(map[string]domainShopee.ShopeeIncome)
	for i, inc := range incomes {
		orderIDs[i] = inc.OrderID
		incomeMap[inc.OrderID] = inc
	}

	orders, err := u.orderRepo.FindByOrderIDs(ctx, orderIDs)
	if err != nil {
		return nil, nil, err
	}

	orderMap := make(map[string]domainShopee.ShopeeOrder)
	for _, ord := range orders {
		orderMap[ord.ID] = ord
	}

	var rows []domainShopee.MatchingItemRow
	orderCountSet := make(map[string]bool)

	for _, inc := range incomes {
		orderCountSet[inc.OrderID] = true
		ord, hasOrder := orderMap[inc.OrderID]

		if !hasOrder || len(ord.Items) == 0 {
			// Income exists without matching Order
			row := domainShopee.MatchingItemRow{
				OrderID:       inc.OrderID,
				TransferDate:  inc.TransferDate,
				OrderNet:      inc.NetAmount,
				AllocatedNet:  inc.NetAmount,
				Profit:        inc.NetAmount,
				MarginPct:     100.0,
				Status:        "MISSING_ORDER",
			}
			if inc.OrderDate != nil {
				row.OrderDate = *inc.OrderDate
			}
			rows = append(rows, row)
			summary.NetReceive += inc.NetAmount
			summary.NetProfit += inc.NetAmount
			continue
		}

		// Calculate Order Gross and costs for all items
		var orderGross float64
		itemCosts := make([]float64, len(ord.Items))
		itemEffectiveCosts := make([]float64, len(ord.Items))

		for idx, item := range ord.Items {
			orderGross += item.SalePrice // BR-01: Line total

			costDate := ord.OrderDate
			if costDate.IsZero() {
				costDate = inc.TransferDate
			}

			effCost, _ := u.costUsecase.GetEffectiveCost(ctx, item.SKU, costDate)
			itemEffectiveCosts[idx] = effCost
			itemCosts[idx] = float64(item.Qty) * effCost // BR-02
		}

		var orderCost float64
		for _, c := range itemCosts {
			orderCost += c
		}

		orderFees := orderGross - inc.NetAmount

		summary.GrossSale += orderGross
		summary.PlatformFees += orderFees
		summary.NetReceive += inc.NetAmount
		summary.TotalCost += orderCost

		// Multi-item allocation
		for idx, item := range ord.Items {
			ratio := 1.0 / float64(len(ord.Items))
			if orderGross > 0 {
				ratio = item.SalePrice / orderGross
			}

			allocatedFee := round2(orderFees * ratio)
			allocatedNet := round2(inc.NetAmount * ratio)
			totalCost := round2(itemCosts[idx])
			profit := round2(allocatedNet - totalCost)

			marginPct := 0.0
			if allocatedNet > 0 {
				marginPct = round2((profit / allocatedNet) * 100)
			}

			status := "OK"
			if itemEffectiveCosts[idx] <= 0 {
				status = "MISSING_COST"
			} else if profit < 0 {
				status = "NEGATIVE_PROFIT"
			}

			rows = append(rows, domainShopee.MatchingItemRow{
				ItemID:        item.ID,
				OrderID:       ord.ID,
				OrderDate:     ord.OrderDate,
				TransferDate:  inc.TransferDate,
				SKU:           item.SKU,
				ProductName:   item.ProductName,
				Qty:           item.Qty,
				EffectiveCost: itemEffectiveCosts[idx],
				TotalCost:     totalCost,
				LineSale:      item.SalePrice,
				OrderGross:    orderGross,
				OrderNet:      inc.NetAmount,
				AllocatedFee:  allocatedFee,
				AllocatedNet:  allocatedNet,
				Profit:        profit,
				MarginPct:     marginPct,
				Status:        status,
			})
		}
	}

	summary.OrderCount = len(orderCountSet)
	summary.ItemCount = len(rows)
	summary.NetProfit = round2(summary.NetReceive - summary.TotalCost)
	if summary.NetReceive > 0 {
		summary.ProfitMarginPct = round2((summary.NetProfit / summary.NetReceive) * 100)
	}

	summary.GrossSale = round2(summary.GrossSale)
	summary.PlatformFees = round2(summary.PlatformFees)
	summary.NetReceive = round2(summary.NetReceive)
	summary.TotalCost = round2(summary.TotalCost)

	// Sort rows by TransferDate ASC, OrderID ASC
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TransferDate.Equal(rows[j].TransferDate) {
			return rows[i].OrderID < rows[j].OrderID
		}
		return rows[i].TransferDate.Before(rows[j].TransferDate)
	})

	return rows, summary, nil
}

func (u *matchingUsecase) GetMonthlyMatching(ctx context.Context, year int, month int) ([]domainShopee.MatchingItemRow, *domainShopee.MonthlySummary, error) {
	return u.calculateMonth(ctx, year, month)
}

func (u *matchingUsecase) ExportMonthlyMatchingCSV(ctx context.Context, year int, month int) ([]byte, string, error) {
	rows, _, err := u.calculateMonth(ctx, year, month)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	// UTF-8 BOM for Excel compatibility with Thai characters
	buf.WriteString("\xEF\xBB\xBF")

	w := csv.NewWriter(&buf)

	headers := []string{
		"Transfer Date",
		"Order ID",
		"Order Date",
		"SKU",
		"Product Name",
		"Qty",
		"Unit Cost",
		"Total Cost",
		"Line Sale",
		"Order Gross",
		"Allocated Fee",
		"Allocated Net Payout",
		"Profit",
		"Margin %",
		"Status",
	}
	if err := w.Write(headers); err != nil {
		return nil, "", err
	}

	for _, r := range rows {
		orderDateStr := ""
		if !r.OrderDate.IsZero() {
			orderDateStr = r.OrderDate.Format("2006-01-02 15:04:05")
		}

		record := []string{
			r.TransferDate.Format("2006-01-02 15:04:05"),
			r.OrderID,
			orderDateStr,
			r.SKU,
			r.ProductName,
			strconv.Itoa(r.Qty),
			fmt.Sprintf("%.2f", r.EffectiveCost),
			fmt.Sprintf("%.2f", r.TotalCost),
			fmt.Sprintf("%.2f", r.LineSale),
			fmt.Sprintf("%.2f", r.OrderGross),
			fmt.Sprintf("%.2f", r.AllocatedFee),
			fmt.Sprintf("%.2f", r.AllocatedNet),
			fmt.Sprintf("%.2f", r.Profit),
			fmt.Sprintf("%.2f", r.MarginPct),
			r.Status,
		}
		if err := w.Write(record); err != nil {
			return nil, "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}

	filename := fmt.Sprintf("shopee_matching_%04d_%02d.csv", year, month)
	return buf.Bytes(), filename, nil
}
