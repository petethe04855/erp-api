package finance

import (
	"context"
	"strings"
	"testing"
	"time"

	domainFinance "chawy-erp-api/internal/domain/finance"
	appErrors "chawy-erp-api/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeSeq numbers documents per type like the shared document_sequences counter.
type fakeSeq struct {
	n     map[string]int
	dates []string
}

func (f *fakeSeq) Generate(_ context.Context, docType string, day *time.Time) (string, error) {
	if f.n == nil {
		f.n = map[string]int{}
	}
	f.n[docType]++
	d := "none"
	if day != nil {
		d = day.Format("2006-01-02")
	}
	f.dates = append(f.dates, docType+"@"+d)
	return docType + "-" + d + "-" + strings.Repeat("0", 2) + string(rune('0'+f.n[docType])), nil
}

// Expense and its journal entry both get sequence numbers (no clock-based
// codes that can collide), and a long AI description does not break the
// journal insert (journal description column is 255 chars).
func TestCreateExpense_UsesSequenceAndTrimsJournalDescription(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	seq := &fakeSeq{}
	uc := NewFinanceUsecaseWithSequence(repo, nil, seq)

	var savedExp *domainFinance.Expense
	var savedJE *domainFinance.JournalEntry
	repo.On("CreateExpense", ctx, mock.Anything).Run(func(a mock.Arguments) {
		savedExp = a.Get(1).(*domainFinance.Expense)
		savedExp.ID = 26
	}).Return(nil)
	repo.On("FindJournalEntryBySource", ctx, "expense", uint(26)).Return(nil, appErrors.ErrNotFound)
	repo.On("GetAccountByCode", ctx, AccountOperatingExpense).Return(&domainFinance.Account{ID: 6, Code: AccountOperatingExpense}, nil)
	repo.On("GetAccountByCode", ctx, AccountCash).Return(&domainFinance.Account{ID: 1, Code: AccountCash}, nil)
	repo.On("CreateJournalEntry", ctx, mock.Anything).Run(func(a mock.Arguments) {
		savedJE = a.Get(1).(*domainFinance.JournalEntry)
	}).Return(nil)

	longDesc := strings.Repeat("1. หมูนุ่มนาว x1 (฿159.00) | ", 20)
	exp, err := uc.CreateExpense(ctx, CreateExpenseInput{
		Date: "2026-11-15", Amount: 1616, Vendor: "สุกีแคมป์ ซาฟารี", Description: longDesc,
	})
	require.NoError(t, err)

	assert.Equal(t, "EXP-2026-11-15-001", exp.Code)
	assert.Equal(t, longDesc, savedExp.Description, "expense keeps the full itemized description")
	require.NotNil(t, savedJE)
	assert.Equal(t, "JE-2026-11-15-001", savedJE.Code)
	assert.Equal(t, "EXP-2026-11-15-001", savedJE.SourceRef)
	assert.LessOrEqual(t, len([]rune(savedJE.Description)), 255)
	assert.Equal(t, []string{"EXP@2026-11-15", "JE@2026-11-15"}, seq.dates)
}

func TestCreateIncome_UsesSequence(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecaseWithSequence(repo, nil, &fakeSeq{})
	repo.On("CreateIncome", ctx, mock.Anything).Return(nil)

	inc, err := uc.CreateIncome(ctx, CreateIncomeInput{Date: "2026-11-20", Amount: 50})
	require.NoError(t, err)
	assert.Equal(t, "INC-2026-11-20-001", inc.Code)
}

// Without a generator (old constructor), codes still differ call to call.
func TestFallbackCodesAreUnique(t *testing.T) {
	uc := &financeUsecase{}
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := uc.nextCode(context.Background(), "EXP", "2026-11-20")
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(c, "EXP-2026-"))
		seen[c] = true
	}
	assert.Greater(t, len(seen), 190)
}
