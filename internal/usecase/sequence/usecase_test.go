package sequence_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	domainSeq "chawy-erp-api/internal/domain/sequence"
	usecaseSeq "chawy-erp-api/internal/usecase/sequence"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memorySequenceRepo struct {
	mu   sync.Mutex
	data map[string]int
}

func newMemorySequenceRepo() *memorySequenceRepo {
	return &memorySequenceRepo{
		data: make(map[string]int),
	}
}

func (m *memorySequenceRepo) NextNumber(ctx context.Context, docType string, dateStr string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", docType, dateStr)
	m.data[key]++
	return m.data[key], nil
}

func TestSequenceUsecase_Generate_DailySequence(t *testing.T) {
	repo := newMemorySequenceRepo()
	uc := usecaseSeq.NewSequenceUsecase(repo)

	loc := usecaseSeq.BangkokLocation()
	date1 := time.Date(2026, 9, 15, 10, 0, 0, 0, loc)
	date2 := time.Date(2026, 9, 16, 9, 0, 0, 0, loc)

	// 1. First GR of date1
	num1, err := uc.Generate(context.Background(), string(domainSeq.TypeGoodsReceive), &date1)
	require.NoError(t, err)
	assert.Equal(t, "GR-2026-09-15-001", num1)

	// 2. Second GR of date1
	num2, err := uc.Generate(context.Background(), string(domainSeq.TypeGoodsReceive), &date1)
	require.NoError(t, err)
	assert.Equal(t, "GR-2026-09-15-002", num2)

	// 3. First SO of date1 should be 001 (separate type sequence)
	so1, err := uc.Generate(context.Background(), string(domainSeq.TypeSalesOrder), &date1)
	require.NoError(t, err)
	assert.Equal(t, "SO-2026-09-15-001", so1)

	// 4. First GR of date2 should reset to 001
	numDate2, err := uc.Generate(context.Background(), string(domainSeq.TypeGoodsReceive), &date2)
	require.NoError(t, err)
	assert.Equal(t, "GR-2026-09-16-001", numDate2)
}

func TestSequenceUsecase_Generate_Over999(t *testing.T) {
	repo := newMemorySequenceRepo()
	uc := usecaseSeq.NewSequenceUsecase(repo)

	loc := usecaseSeq.BangkokLocation()
	date := time.Date(2026, 9, 15, 10, 0, 0, 0, loc)

	repo.data["INV:2026-09-15"] = 999

	num, err := uc.Generate(context.Background(), string(domainSeq.TypeInvoice), &date)
	require.NoError(t, err)
	assert.Equal(t, "INV-2026-09-15-1000", num)
}

func TestSequenceUsecase_Generate_InvalidType(t *testing.T) {
	repo := newMemorySequenceRepo()
	uc := usecaseSeq.NewSequenceUsecase(repo)

	_, err := uc.Generate(context.Background(), "UNKNOWN", nil)
	assert.Error(t, err)
}
