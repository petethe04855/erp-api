package finance

import (
	"context"
	"sort"

	domainFinance "chawy-erp-api/internal/domain/finance"
)

func (u *financeUsecase) GetGeneralLedger(ctx context.Context, from, to string, accountID uint) ([]GeneralLedgerRow, error) {
	lines, err := u.repo.GetJournalLinesForReport(ctx, domainFinance.ReportFilter{
		From:      from,
		To:        to,
		AccountID: accountID,
	})
	if err != nil {
		return nil, err
	}

	openingMap, err := u.repo.GetOpeningBalances(ctx, from)
	if err != nil {
		return nil, err
	}

	runningMap := make(map[string]float64)
	for code, val := range openingMap {
		runningMap[code] = val
	}

	rows := make([]GeneralLedgerRow, 0, len(lines))
	for _, l := range lines {
		// Calculate running balance for this account
		runningMap[l.AccountCode] += l.Debit - l.Credit

		rows = append(rows, GeneralLedgerRow{
			AccountCode:    l.AccountCode,
			AccountName:    l.AccountName,
			Debit:          l.Debit,
			Credit:         l.Credit,
			SKU:            l.SKU,
			Lot:            l.Lot,
			Channel:        l.Channel,
			OpeningBalance: openingMap[l.AccountCode],
			RunningBalance: runningMap[l.AccountCode],
		})
	}

	return rows, nil
}

func (u *financeUsecase) GetTrialBalance(ctx context.Context, from, to string) ([]TrialBalanceRow, error) {
	accounts, err := u.repo.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}

	lines, err := u.repo.GetJournalLinesForReport(ctx, domainFinance.ReportFilter{
		From: from,
		To:   to,
	})
	if err != nil {
		return nil, err
	}

	openingMap, err := u.repo.GetOpeningBalances(ctx, from)
	if err != nil {
		return nil, err
	}

	type accStats struct {
		debit  float64
		credit float64
	}
	periodStats := make(map[string]*accStats)
	for _, l := range lines {
		if _, ok := periodStats[l.AccountCode]; !ok {
			periodStats[l.AccountCode] = &accStats{}
		}
		periodStats[l.AccountCode].debit += l.Debit
		periodStats[l.AccountCode].credit += l.Credit
	}

	rows := make([]TrialBalanceRow, 0, len(accounts))
	for _, acc := range accounts {
		openNet := openingMap[acc.Code]
		openDebit := 0.0
		openCredit := 0.0
		if openNet > 0 {
			openDebit = openNet
		} else if openNet < 0 {
			openCredit = -openNet
		}

		curDebit := 0.0
		curCredit := 0.0
		if stats, ok := periodStats[acc.Code]; ok {
			curDebit = stats.debit
			curCredit = stats.credit
		}

		endNet := openNet + (curDebit - curCredit)
		endDebit := 0.0
		endCredit := 0.0
		if endNet > 0 {
			endDebit = endNet
		} else if endNet < 0 {
			endCredit = -endNet
		}

		rows = append(rows, TrialBalanceRow{
			AccountCode:   acc.Code,
			AccountName:   acc.Name,
			AccountType:   string(acc.Type),
			OpeningDebit:  openDebit,
			OpeningCredit: openCredit,
			Debit:         curDebit,
			Credit:        curCredit,
			EndingDebit:   endDebit,
			EndingCredit:  endCredit,
		})
	}

	return rows, nil
}

func (u *financeUsecase) GetProfitAndLoss(ctx context.Context, from, to, channel string) (*ProfitAndLossReport, error) {
	lines, err := u.repo.GetJournalLinesForReport(ctx, domainFinance.ReportFilter{
		From:    from,
		To:      to,
		Channel: channel,
	})
	if err != nil {
		return nil, err
	}

	report := &ProfitAndLossReport{
		From:              from,
		To:                to,
		RevenueByChannel:  make(map[string]float64),
		ExpenseByCategory: make(map[string]float64),
	}

	for _, l := range lines {
		// 4000 series = Revenue (Credit increases revenue)
		if len(l.AccountCode) > 0 && l.AccountCode[0] == '4' {
			rev := l.Credit - l.Debit
			report.Revenue += rev
			ch := l.Channel
			if ch == "" {
				ch = "General"
			}
			report.RevenueByChannel[ch] += rev
		}

		// 5000 series = COGS (Debit increases cost)
		if len(l.AccountCode) > 0 && l.AccountCode[0] == '5' {
			cost := l.Debit - l.Credit
			report.COGS += cost
		}

		// 6000 series = Operating Expenses (Debit increases expense)
		if len(l.AccountCode) > 0 && l.AccountCode[0] == '6' {
			exp := l.Debit - l.Credit
			report.OperatingExpense += exp
			cat := l.AccountName
			if cat == "" {
				cat = "General Expense"
			}
			report.ExpenseByCategory[cat] += exp
		}
	}

	report.GrossProfit = report.Revenue - report.COGS
	report.NetProfit = report.GrossProfit - report.OperatingExpense

	return report, nil
}

func (u *financeUsecase) GetRevenueByChannel(ctx context.Context, from, to string) (*RevenueByChannelReport, error) {
	pnl, err := u.GetProfitAndLoss(ctx, from, to, "")
	if err != nil {
		return nil, err
	}

	report := &RevenueByChannelReport{
		From:      from,
		To:        to,
		Total:     pnl.Revenue,
		ByChannel: make([]RevenueByChannelItem, 0, len(pnl.RevenueByChannel)),
	}

	for ch, amt := range pnl.RevenueByChannel {
		pct := 0.0
		if pnl.Revenue > 0 {
			pct = (amt / pnl.Revenue) * 100
		}
		report.ByChannel = append(report.ByChannel, RevenueByChannelItem{
			Channel:    ch,
			Amount:     amt,
			Percentage: pct,
		})
	}

	sort.Slice(report.ByChannel, func(i, j int) bool {
		return report.ByChannel[i].Amount > report.ByChannel[j].Amount
	})

	return report, nil
}
