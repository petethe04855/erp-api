package finance

import (
	"context"
	"strings"
	"testing"
	"time"

	domainFinance "chawy-erp-api/internal/domain/finance"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateIncome_RejectsNonPositiveAmount(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	for _, amount := range []float64{0, -10} {
		_, err := uc.CreateIncome(ctx, CreateIncomeInput{Amount: amount})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "greater than zero")
	}
	repo.AssertNotCalled(t, "CreateIncome", mock.Anything, mock.Anything)
}

func TestCreateIncome_AppliesDefaults(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	repo.On("CreateIncome", ctx, mock.AnythingOfType("*finance.Income")).Return(nil)

	inc, err := uc.CreateIncome(ctx, CreateIncomeInput{Amount: 250.5})
	require.NoError(t, err)
	require.NotNil(t, inc)

	assert.Equal(t, time.Now().Format("2006-01-02"), inc.Date)
	assert.Equal(t, domainFinance.IncomeCategoryOther, inc.Category)
	assert.Equal(t, domainFinance.ExpenseChannelGeneral, inc.Channel)
	assert.Equal(t, "system", inc.CreatedBy)
	assert.Equal(t, 250.5, inc.Amount)
	assert.True(t, strings.HasPrefix(inc.Code, "INC-"), "income code must start with INC-, got %q", inc.Code)
	repo.AssertExpectations(t)
}

func TestCreateIncome_KeepsProvidedValues(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)

	repo.On("CreateIncome", ctx, mock.AnythingOfType("*finance.Income")).Return(nil)

	inc, err := uc.CreateIncome(ctx, CreateIncomeInput{
		Date:        "2026-11-01",
		Category:    domainFinance.IncomeCategorySalesLine,
		Channel:     domainFinance.ExpenseChannelLine,
		Amount:      1200,
		ProductName: "อกไก่",
		SlipURL:     "/uploads/incomes/income-1.jpg",
		CreatedBy:   "staff@chawy.test",
	})
	require.NoError(t, err)

	assert.Equal(t, "2026-11-01", inc.Date)
	assert.Equal(t, domainFinance.IncomeCategorySalesLine, inc.Category)
	assert.Equal(t, domainFinance.ExpenseChannelLine, inc.Channel)
	assert.Equal(t, "staff@chawy.test", inc.CreatedBy)
	assert.Equal(t, "/uploads/incomes/income-1.jpg", inc.SlipURL)
	assert.Equal(t, "อกไก่", inc.ProductName)
	repo.AssertExpectations(t)
}

// AI scans can return long joined vendors / order numbers; they are capped to
// the column size (by characters, Thai-safe) instead of failing the insert,
// while the itemized description is kept in full.
func TestCreateIncome_ClipsLongScannedFields(t *testing.T) {
	ctx := context.Background()
	repo := new(mockFinanceRepo)
	uc := NewFinanceUsecase(repo, nil)
	repo.On("CreateIncome", ctx, mock.AnythingOfType("*finance.Income")).Return(nil)

	longThai := strings.Repeat("ร้านค้า", 40) // 280 runes
	longDesc := strings.Repeat("1. สินค้า x1 (฿10) | ", 30)

	inc, err := uc.CreateIncome(ctx, CreateIncomeInput{
		Amount:      10,
		Vendor:      longThai,
		InvoiceRef:  strings.Repeat("ORD123456 / ", 20),
		ProductName: longThai,
		Description: longDesc,
	})
	require.NoError(t, err)

	assert.Len(t, []rune(inc.Vendor), maxVendor)
	assert.LessOrEqual(t, len([]rune(inc.InvoiceRef)), maxInvoiceRef)
	assert.Len(t, []rune(inc.ProductName), maxProductName)
	assert.Equal(t, longDesc, inc.Description)
}
