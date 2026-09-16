package postgres

import (
	"context"
	"fmt"
	"time"

	domainFinance "chawy-erp-api/internal/domain/finance"
	"chawy-erp-api/pkg/database"

	"gorm.io/gorm"
)

type financeRepository struct {
	db *gorm.DB
}

func NewFinanceRepository(db *gorm.DB) domainFinance.Repository {
	return &financeRepository{db: db}
}

func (r *financeRepository) getDB(ctx context.Context) *gorm.DB {
	return database.GetDBFromContext(ctx, r.db)
}

// Accounts
func (r *financeRepository) GetAccountByCode(ctx context.Context, code string) (*domainFinance.Account, error) {
	var acc domainFinance.Account
	err := r.getDB(ctx).Where("code = ? AND is_active = ?", code, true).First(&acc).Error
	if err != nil {
		return nil, err
	}
	return &acc, nil
}

func (r *financeRepository) ListAccounts(ctx context.Context) ([]domainFinance.Account, error) {
	var accounts []domainFinance.Account
	err := r.getDB(ctx).Where("is_active = ?", true).Order("code ASC").Find(&accounts).Error
	return accounts, err
}

func (r *financeRepository) GetAccountMapping(ctx context.Context, key string) (*domainFinance.AccountMapping, error) {
	var mapping domainFinance.AccountMapping
	err := r.getDB(ctx).Where("mapping_key = ? AND is_active = ?", key, true).First(&mapping).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

func (r *financeRepository) ListAccountMappings(ctx context.Context) ([]domainFinance.AccountMapping, error) {
	var mappings []domainFinance.AccountMapping
	err := r.getDB(ctx).Where("is_active = ?", true).Order("mapping_key ASC").Find(&mappings).Error
	return mappings, err
}

func (r *financeRepository) UpsertAccountMapping(ctx context.Context, mapping *domainFinance.AccountMapping) error {
	var existing domainFinance.AccountMapping
	db := r.getDB(ctx)
	err := db.Where("mapping_key = ?", mapping.MappingKey).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(mapping).Error
	} else if err != nil {
		return err
	}
	existing.AccountCode = mapping.AccountCode
	existing.Description = mapping.Description
	existing.IsActive = mapping.IsActive
	return db.Save(&existing).Error
}

// Journal
func (r *financeRepository) CreateJournalEntry(ctx context.Context, entry *domainFinance.JournalEntry) error {
	return r.getDB(ctx).Create(entry).Error
}

func (r *financeRepository) FindJournalEntries(ctx context.Context, filter domainFinance.JournalFilter) ([]domainFinance.JournalEntry, int64, error) {
	var entries []domainFinance.JournalEntry
	var total int64

	q := r.getDB(ctx).Model(&domainFinance.JournalEntry{}).Preload("Lines")

	if filter.From != "" {
		q = q.Where("date >= ?", filter.From)
	}
	if filter.To != "" {
		q = q.Where("date <= ?", filter.To)
	}
	if filter.SourceType != "" {
		q = q.Where("source_type = ?", filter.SourceType)
	}
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		q = q.Where("code ILIKE ? OR source_ref ILIKE ? OR description ILIKE ?", searchPattern, searchPattern, searchPattern)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	offset := (filter.Page - 1) * filter.Limit

	err := q.Order("date DESC, id DESC").Offset(offset).Limit(filter.Limit).Find(&entries).Error
	return entries, total, err
}

func (r *financeRepository) FindJournalEntryByID(ctx context.Context, id uint) (*domainFinance.JournalEntry, error) {
	var entry domainFinance.JournalEntry
	err := r.getDB(ctx).Preload("Lines").First(&entry, id).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *financeRepository) FindJournalEntryBySource(ctx context.Context, sourceType string, sourceID uint) (*domainFinance.JournalEntry, error) {
	var entry domainFinance.JournalEntry
	err := r.getDB(ctx).Preload("Lines").Where("source_type = ? AND source_id = ?", sourceType, sourceID).First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *financeRepository) GetJournalLinesForReport(ctx context.Context, filter domainFinance.ReportFilter) ([]domainFinance.JournalLine, error) {
	var lines []domainFinance.JournalLine
	q := r.getDB(ctx).Model(&domainFinance.JournalLine{}).
		Joins("JOIN journal_entries ON journal_entries.id = journal_lines.journal_entry_id").
		Where("journal_entries.status = ?", domainFinance.JournalStatusPosted)

	if filter.From != "" {
		q = q.Where("journal_entries.date >= ?", filter.From)
	}
	if filter.To != "" {
		q = q.Where("journal_entries.date <= ?", filter.To)
	}
	if filter.AccountID != 0 {
		q = q.Where("journal_lines.account_id = ?", filter.AccountID)
	}
	if filter.Channel != "" {
		q = q.Where("journal_lines.channel = ?", filter.Channel)
	}

	err := q.Order("journal_entries.date ASC, journal_entries.id ASC, journal_lines.id ASC").Find(&lines).Error
	return lines, err
}

func (r *financeRepository) GetOpeningBalances(ctx context.Context, beforeDate string) (map[string]float64, error) {
	balances := make(map[string]float64)
	if beforeDate == "" {
		return balances, nil
	}

	type Result struct {
		AccountCode string
		NetDebit    float64
	}

	var results []Result
	err := r.getDB(ctx).Model(&domainFinance.JournalLine{}).
		Select("journal_lines.account_code, SUM(journal_lines.debit - journal_lines.credit) as net_debit").
		Joins("JOIN journal_entries ON journal_entries.id = journal_lines.journal_entry_id").
		Where("journal_entries.status = ? AND journal_entries.date < ?", domainFinance.JournalStatusPosted, beforeDate).
		Group("journal_lines.account_code").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	for _, res := range results {
		balances[res.AccountCode] = res.NetDebit
	}
	return balances, nil
}

// Expenses
func (r *financeRepository) CreateExpense(ctx context.Context, exp *domainFinance.Expense) error {
	return r.getDB(ctx).Create(exp).Error
}

func (r *financeRepository) UpdateExpense(ctx context.Context, exp *domainFinance.Expense) error {
	return r.getDB(ctx).Save(exp).Error
}

func (r *financeRepository) DeleteExpense(ctx context.Context, id uint) error {
	return r.getDB(ctx).Delete(&domainFinance.Expense{}, id).Error
}

func (r *financeRepository) GetExpenseByID(ctx context.Context, id uint) (*domainFinance.Expense, error) {
	var exp domainFinance.Expense
	err := r.getDB(ctx).First(&exp, id).Error
	if err != nil {
		return nil, err
	}
	return &exp, nil
}

func (r *financeRepository) ListExpenses(ctx context.Context, filter domainFinance.ExpenseFilter) ([]domainFinance.Expense, int64, error) {
	var expenses []domainFinance.Expense
	var total int64

	q := r.getDB(ctx).Model(&domainFinance.Expense{})

	if filter.From != "" {
		q = q.Where("date >= ?", filter.From)
	}
	if filter.To != "" {
		q = q.Where("date <= ?", filter.To)
	}
	if filter.Category != "" {
		q = q.Where("category = ?", filter.Category)
	}
	if filter.Channel != "" {
		q = q.Where("channel = ?", filter.Channel)
	}
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		q = q.Where("code ILIKE ? OR vendor ILIKE ? OR invoice_ref ILIKE ? OR description ILIKE ?", searchPattern, searchPattern, searchPattern, searchPattern)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	offset := (filter.Page - 1) * filter.Limit

	err := q.Order("date DESC, id DESC").Offset(offset).Limit(filter.Limit).Find(&expenses).Error
	return expenses, total, err
}

// NextJournalCode generates JE-YYYY-XXXX
func NextJournalCode(db *gorm.DB, dateStr string) (string, error) {
	year := time.Now().Format("2006")
	if len(dateStr) >= 4 {
		year = dateStr[:4]
	}
	prefix := fmt.Sprintf("JE-%s-", year)

	var lastEntry domainFinance.JournalEntry
	err := db.Where("code LIKE ?", prefix+"%").Order("code DESC").First(&lastEntry).Error
	seq := 1
	if err == nil {
		var lastSeq int
		if _, scanErr := fmt.Sscanf(lastEntry.Code, prefix+"%d", &lastSeq); scanErr == nil {
			seq = lastSeq + 1
		}
	}
	return fmt.Sprintf("%s%05d", prefix, seq), nil
}

// NextExpenseCode generates EXP-YYYY-XXXX
func NextExpenseCode(db *gorm.DB, dateStr string) (string, error) {
	year := time.Now().Format("2006")
	if len(dateStr) >= 4 {
		year = dateStr[:4]
	}
	prefix := fmt.Sprintf("EXP-%s-", year)

	var lastExp domainFinance.Expense
	err := db.Where("code LIKE ?", prefix+"%").Order("code DESC").First(&lastExp).Error
	seq := 1
	if err == nil {
		var lastSeq int
		if _, scanErr := fmt.Sscanf(lastExp.Code, prefix+"%d", &lastSeq); scanErr == nil {
			seq = lastSeq + 1
		}
	}
	return fmt.Sprintf("%s%05d", prefix, seq), nil
}
