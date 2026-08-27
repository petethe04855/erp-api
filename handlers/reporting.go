package handlers

import (
	"sort"
	"time"

	"chawy-erp-api/database"
	"chawy-erp-api/models"
	"github.com/gofiber/fiber/v2"
)

func reportDateRange(c *fiber.Ctx) (string, string) {
	from, to := c.Query("from"), c.Query("to")
	if month := c.Query("month"); month != "" {
		from = month + "-01"
		if parsed, err := time.Parse("2006-01-02", from); err == nil {
			to = parsed.AddDate(0, 1, 0).AddDate(0, 0, -1).Format("2006-01-02")
		}
	}
	return from, to
}

func journalRangeQuery(c *fiber.Ctx) []models.JournalEntry {
	from, to := reportDateRange(c)
	query := database.DB.Preload("Lines").Where("status = ?", "Posted")
	if from != "" {
		query = query.Where("date >= ?", from)
	}
	if to != "" {
		query = query.Where("date <= ?", to)
	}
	var entries []models.JournalEntry
	query.Order("date ASC, id ASC").Find(&entries)
	return entries
}

// GET /api/reports/general-ledger
func GeneralLedgerReport(c *fiber.Ctx) error {
	entries := journalRangeQuery(c)
	type row struct {
		Date, JournalCode, SourceType, SourceRef, AccountCode, AccountName, Description, SKU, Lot, Channel string
		Debit, Credit, RunningBalance                                                                      float64
	}
	rows := make([]row, 0)
	running := map[string]float64{}
	for _, entry := range entries {
		for _, line := range entry.Lines {
			running[line.AccountCode] += line.Debit - line.Credit
			rows = append(rows, row{entry.Date, entry.Code, entry.SourceType, entry.SourceRef, line.AccountCode, line.AccountName, entry.Description, line.SKU, line.Lot, line.Channel, line.Debit, line.Credit, running[line.AccountCode]})
		}
	}
	return c.JSON(rows)
}

// GET /api/reports/trial-balance
func TrialBalanceReport(c *fiber.Ctx) error {
	from, to := reportDateRange(c)
	entries := journalRangeQuery(c)
	type balance struct {
		AccountCode, AccountName, AccountType                     string
		OpeningDebit, OpeningCredit, Debit, Credit, EndingBalance float64
		BalanceSide                                               string
	}
	byCode := map[string]*balance{}
	// Include all active accounts so a zero-activity account is not silently omitted.
	var accounts []models.Account
	database.DB.Where("is_active = ?", true).Find(&accounts)
	for _, account := range accounts {
		byCode[account.Code] = &balance{AccountCode: account.Code, AccountName: account.Name, AccountType: account.Type}
	}
	// Opening balance is the posted movement before the selected period.
	if from != "" {
		var prior []models.JournalEntry
		database.DB.Preload("Lines").Where("status = ? AND date < ?", "Posted", from).Find(&prior)
		for _, entry := range prior {
			for _, line := range entry.Lines {
				item := byCode[line.AccountCode]
				if item == nil {
					item = &balance{AccountCode: line.AccountCode, AccountName: line.AccountName}
					byCode[line.AccountCode] = item
				}
				item.OpeningDebit += line.Debit
				item.OpeningCredit += line.Credit
			}
		}
	}
	for _, entry := range entries {
		for _, line := range entry.Lines {
			item := byCode[line.AccountCode]
			if item == nil {
				item = &balance{AccountCode: line.AccountCode, AccountName: line.AccountName}
				byCode[line.AccountCode] = item
			}
			item.Debit += line.Debit
			item.Credit += line.Credit
		}
	}
	database.DB.Find(&accounts)
	for _, account := range accounts {
		if item := byCode[account.Code]; item != nil {
			item.AccountType = account.Type
		}
	}
	rows := make([]balance, 0, len(byCode))
	totalDebit, totalCredit := 0.0, 0.0
	openingDebit, openingCredit := 0.0, 0.0
	for _, item := range byCode {
		openingDebit += item.OpeningDebit
		openingCredit += item.OpeningCredit
		item.EndingBalance = item.OpeningDebit - item.OpeningCredit + item.Debit - item.Credit
		item.BalanceSide = "Debit"
		if item.EndingBalance < 0 {
			item.EndingBalance = -item.EndingBalance
			item.BalanceSide = "Credit"
		}
		totalDebit += item.Debit
		totalCredit += item.Credit
		rows = append(rows, *item)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].AccountCode < rows[j].AccountCode })
	return c.JSON(fiber.Map{"rows": rows, "totalDebit": totalDebit, "totalCredit": totalCredit, "openingDebit": openingDebit, "openingCredit": openingCredit, "endingDebit": totalDebit + openingDebit, "endingCredit": totalCredit + openingCredit, "from": from, "to": to, "balanced": absFloat(totalDebit-totalCredit) < 0.005})
}

// GET /api/reports/inventory-valuation
func InventoryValuationReport(c *fiber.Ctx) error {
	type row struct {
		SKU, ProductName, Lot, ExpiryDate string
		RemainingQty                      int
		UnitCost, Value                   float64
	}
	var lots []models.StockLot
	database.DB.Where("remaining_qty > 0").Order("sku, expiry_date, id").Find(&lots)
	var products []models.Product
	database.DB.Find(&products)
	names := map[string]string{}
	for _, product := range products {
		names[product.SKU] = product.Name
	}
	rows := make([]row, 0, len(lots))
	totalQty := 0
	totalValue := 0.0
	for _, lot := range lots {
		value := float64(lot.RemainingQty) * lot.LandedUnitCost
		rows = append(rows, row{lot.SKU, names[lot.SKU], lot.Lot, lot.ExpiryDate, lot.RemainingQty, lot.LandedUnitCost, value})
		totalQty += lot.RemainingQty
		totalValue += value
	}
	return c.JSON(fiber.Map{"rows": rows, "totalQty": totalQty, "totalValue": totalValue})
}

// GET /api/reports/financial-summary
func FinancialSummaryReport(c *fiber.Ctx) error {
	entries := journalRangeQuery(c)
	amounts := map[string]float64{}
	for _, entry := range entries {
		for _, line := range entry.Lines {
			amounts[line.AccountCode] += line.Credit - line.Debit
		}
	}
	return c.JSON(summarizeAccountAmounts(amounts))
}

func summarizeAccountAmounts(amounts map[string]float64) fiber.Map {
	revenue := amounts["4000"] + amounts["5100"]
	cogs := -amounts["5000"]
	damageLoss := -amounts["5200"]
	opex := -amounts["6000"]
	grossProfit := revenue - cogs
	return fiber.Map{
		"revenue": revenue, "salesRevenue": amounts["4000"], "salesReturns": -amounts["5100"],
		"cogs": cogs, "grossProfit": grossProfit, "damageLoss": damageLoss, "operatingExpenses": opex,
		"netProfit":          grossProfit - damageLoss - opex,
		"accountsReceivable": -amounts["1200"], "inventory": -amounts["1300"], "cash": -amounts["1100"], "bank": -amounts["1110"],
	}
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
