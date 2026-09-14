package salesreturn_test

import (
	"context"
	"testing"

	domainOrder "chawy-erp-api/internal/domain/order"
	domainReturn "chawy-erp-api/internal/domain/salesreturn"
	domainStock "chawy-erp-api/internal/domain/stock"
	domainSKU "chawy-erp-api/internal/domain/sku"
	usecaseReturn "chawy-erp-api/internal/usecase/salesreturn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockReturnRepository
type MockReturnRepository struct {
	mock.Mock
}

func (m *MockReturnRepository) Create(ctx context.Context, ret *domainReturn.SalesReturn) error {
	args := m.Called(ctx, ret)
	ret.ID = 101
	return args.Error(0)
}
func (m *MockReturnRepository) FindByID(ctx context.Context, id uint) (*domainReturn.SalesReturn, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainReturn.SalesReturn), args.Error(1)
}
func (m *MockReturnRepository) FindByIDForUpdate(ctx context.Context, id uint) (*domainReturn.SalesReturn, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainReturn.SalesReturn), args.Error(1)
}
func (m *MockReturnRepository) FindByReturnNo(ctx context.Context, returnNo string) (*domainReturn.SalesReturn, error) {
	args := m.Called(ctx, returnNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainReturn.SalesReturn), args.Error(1)
}
func (m *MockReturnRepository) FindAll(ctx context.Context, query domainReturn.Query) ([]domainReturn.SalesReturn, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]domainReturn.SalesReturn), args.Get(1).(int64), args.Error(2)
}
func (m *MockReturnRepository) Update(ctx context.Context, ret *domainReturn.SalesReturn) error {
	args := m.Called(ctx, ret)
	return args.Error(0)
}
func (m *MockReturnRepository) UpdateLines(ctx context.Context, lines []domainReturn.SalesReturnLine) error {
	args := m.Called(ctx, lines)
	return args.Error(0)
}
func (m *MockReturnRepository) DeleteLines(ctx context.Context, returnID uint) error {
	args := m.Called(ctx, returnID)
	return args.Error(0)
}
func (m *MockReturnRepository) CreateLines(ctx context.Context, lines []domainReturn.SalesReturnLine) error {
	args := m.Called(ctx, lines)
	return args.Error(0)
}
func (m *MockReturnRepository) CountReturnedQtyByOrderAndSKU(ctx context.Context, orderID uint, excludeReturnID uint) (map[string]int, error) {
	args := m.Called(ctx, orderID, excludeReturnID)
	return args.Get(0).(map[string]int), args.Error(1)
}

// MockOrderRepository
type MockOrderRepository struct {
	mock.Mock
}

func (m *MockOrderRepository) Create(ctx context.Context, order *domainOrder.Order) error {
	return m.Called(ctx, order).Error(0)
}
func (m *MockOrderRepository) FindByID(ctx context.Context, id uint) (*domainOrder.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainOrder.Order), args.Error(1)
}
func (m *MockOrderRepository) FindByIDForUpdate(ctx context.Context, id uint) (*domainOrder.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainOrder.Order), args.Error(1)
}
func (m *MockOrderRepository) FindByOrderNo(ctx context.Context, orderNo string) (*domainOrder.Order, error) {
	args := m.Called(ctx, orderNo)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainOrder.Order), args.Error(1)
}
func (m *MockOrderRepository) FindAll(ctx context.Context, query domainOrder.Query) ([]domainOrder.Order, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]domainOrder.Order), args.Get(1).(int64), args.Error(2)
}
func (m *MockOrderRepository) Update(ctx context.Context, order *domainOrder.Order) error {
	return m.Called(ctx, order).Error(0)
}
func (m *MockOrderRepository) UpdateStatus(ctx context.Context, id uint, status domainOrder.Status) error {
	return m.Called(ctx, id, status).Error(0)
}

// MockSKURepository
type MockSKURepository struct {
	mock.Mock
}

func (m *MockSKURepository) Create(ctx context.Context, s *domainSKU.SKU) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockSKURepository) FindByID(ctx context.Context, id uint) (*domainSKU.SKU, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainSKU.SKU), args.Error(1)
}
func (m *MockSKURepository) FindBySKU(ctx context.Context, sku string) (*domainSKU.SKU, error) {
	args := m.Called(ctx, sku)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainSKU.SKU), args.Error(1)
}
func (m *MockSKURepository) FindAll(ctx context.Context, query domainSKU.Query) ([]domainSKU.SKU, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]domainSKU.SKU), args.Get(1).(int64), args.Error(2)
}
func (m *MockSKURepository) Update(ctx context.Context, s *domainSKU.SKU) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockSKURepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockSKURepository) ExistsBySKU(ctx context.Context, skuCode string) (bool, error) {
	args := m.Called(ctx, skuCode)
	return args.Bool(0), args.Error(1)
}

// MockStockRepository
type MockStockRepository struct {
	mock.Mock
}

func (m *MockStockRepository) GetBySKUID(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error) {
	args := m.Called(ctx, skuID, warehouseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainStock.Stock), args.Error(1)
}
func (m *MockStockRepository) GetBySKUIDForUpdate(ctx context.Context, skuID, warehouseID uint) (*domainStock.Stock, error) {
	args := m.Called(ctx, skuID, warehouseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainStock.Stock), args.Error(1)
}
func (m *MockStockRepository) FindAll(ctx context.Context, query domainStock.Query) ([]domainStock.Stock, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]domainStock.Stock), args.Get(1).(int64), args.Error(2)
}
func (m *MockStockRepository) FindAllBySKU(ctx context.Context, query domainStock.StockBySKUQuery) ([]domainStock.StockBySKU, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]domainStock.StockBySKU), args.Get(1).(int64), args.Error(2)
}
func (m *MockStockRepository) UpdateQuantity(ctx context.Context, skuID, warehouseID uint, delta int) (*domainStock.Stock, error) {
	args := m.Called(ctx, skuID, warehouseID, delta)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainStock.Stock), args.Error(1)
}
func (m *MockStockRepository) ReserveStock(ctx context.Context, skuID, warehouseID uint, qty int) (*domainStock.Stock, error) {
	args := m.Called(ctx, skuID, warehouseID, qty)
	return args.Get(0).(*domainStock.Stock), args.Error(1)
}
func (m *MockStockRepository) ReleaseStock(ctx context.Context, skuID, warehouseID uint, qty int) (*domainStock.Stock, error) {
	args := m.Called(ctx, skuID, warehouseID, qty)
	return args.Get(0).(*domainStock.Stock), args.Error(1)
}
func (m *MockStockRepository) CreateMovement(ctx context.Context, movement *domainStock.StockMovement) error {
	return m.Called(ctx, movement).Error(0)
}
func (m *MockStockRepository) GetMovements(ctx context.Context, skuID uint, page, limit int) ([]domainStock.StockMovement, int64, error) {
	args := m.Called(ctx, skuID, page, limit)
	return args.Get(0).([]domainStock.StockMovement), args.Get(1).(int64), args.Error(2)
}
func (m *MockStockRepository) CreateLot(ctx context.Context, lot *domainStock.StockLot) error {
	return m.Called(ctx, lot).Error(0)
}
func (m *MockStockRepository) GetAvailableLotsForUpdate(ctx context.Context, skuID, warehouseID uint) ([]domainStock.StockLot, error) {
	args := m.Called(ctx, skuID, warehouseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domainStock.StockLot), args.Error(1)
}
func (m *MockStockRepository) DeductLotQuantity(ctx context.Context, lotID uint, qty int) (*domainStock.StockLot, error) {
	args := m.Called(ctx, lotID, qty)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainStock.StockLot), args.Error(1)
}
func (m *MockStockRepository) FindLotsBySKU(ctx context.Context, skuID, warehouseID uint) ([]domainStock.StockLot, error) {
	args := m.Called(ctx, skuID, warehouseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domainStock.StockLot), args.Error(1)
}

func TestSalesReturn_CreateAndEnforceReturnable(t *testing.T) {
	mockRet := new(MockReturnRepository)
	mockOrder := new(MockOrderRepository)
	mockSKU := new(MockSKURepository)
	mockStock := new(MockStockRepository)

	uc := usecaseReturn.NewSalesReturnUsecase(
		nil,
		mockRet,
		mockOrder,
		nil,
		mockSKU,
		mockStock,
		nil,
		nil,
	)

	orderID := uint(50)
	order := &domainOrder.Order{
		ID:           orderID,
		OrderNo:      "SO-2026-0001",
		CustomerID:   10,
		CustomerName: "Customer A",
		Status:       domainOrder.StatusShipped,
		Items: []domainOrder.OrderItem{
			{SKU: "SKU-A", Quantity: 5, Price: 100},
		},
	}

	mockOrder.On("FindByID", mock.Anything, orderID).Return(order, nil)
	// Already returned 3 out of 5
	mockRet.On("CountReturnedQtyByOrderAndSKU", mock.Anything, orderID, uint(0)).Return(map[string]int{
		"SKU-A": 3,
	}, nil)

	// Attempt to return 3 (which exceeds returnable 2) -> must fail
	_, err := uc.Create(context.Background(), usecaseReturn.CreateReturnInput{
		ReturnType:  domainReturn.ReturnTypeCustomer,
		OrderID:     &orderID,
		WarehouseID: 1,
		Lines: []usecaseReturn.CreateReturnLineInput{
			{SKU: "SKU-A", Quantity: 3, Condition: domainReturn.ConditionGood},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds available returnable quantity")

	// Attempt to return 2 (valid) -> must succeed
	mockRet.On("Create", mock.Anything, mock.Anything).Return(nil)
	ret, err := uc.Create(context.Background(), usecaseReturn.CreateReturnInput{
		ReturnType:  domainReturn.ReturnTypeCustomer,
		OrderID:     &orderID,
		WarehouseID: 1,
		Lines: []usecaseReturn.CreateReturnLineInput{
			{SKU: "SKU-A", Quantity: 2, Condition: domainReturn.ConditionGood},
		},
	})
	assert.NoError(t, err)
	assert.NotNil(t, ret)
	assert.Equal(t, domainReturn.StatusDraft, ret.Status)
	assert.Equal(t, 200.0, ret.Subtotal)
}

func TestSalesReturn_CompleteRestocksGoodConditionOnly(t *testing.T) {
	mockRet := new(MockReturnRepository)
	mockOrder := new(MockOrderRepository)
	mockSKU := new(MockSKURepository)
	mockStock := new(MockStockRepository)

	uc := usecaseReturn.NewSalesReturnUsecase(
		nil,
		mockRet,
		mockOrder,
		nil,
		mockSKU,
		mockStock,
		nil,
		nil,
	)

	returnID := uint(101)
	retDoc := &domainReturn.SalesReturn{
		ID:          returnID,
		ReturnNo:    "RT-2026-0001",
		Status:      domainReturn.StatusApproved,
		WarehouseID: 1,
		Lines: []domainReturn.SalesReturnLine{
			{
				ID:        1,
				ReturnID:  returnID,
				SKU:       "SKU-GOOD",
				Quantity:  2,
				Condition: domainReturn.ConditionGood,
				Restock:   true,
			},
			{
				ID:        2,
				ReturnID:  returnID,
				SKU:       "SKU-DAMAGED",
				Quantity:  1,
				Condition: domainReturn.ConditionDamaged,
				Restock:   false,
			},
		},
	}

	mockRet.On("FindByIDForUpdate", mock.Anything, returnID).Return(retDoc, nil)
	mockRet.On("Update", mock.Anything, mock.Anything).Return(nil)
	mockRet.On("UpdateLines", mock.Anything, mock.Anything).Return(nil)

	skuGood := &domainSKU.SKU{ID: 1, SKU: "SKU-GOOD", CostPrice: 50}
	skuDamaged := &domainSKU.SKU{ID: 2, SKU: "SKU-DAMAGED", CostPrice: 50}

	mockSKU.On("FindBySKU", mock.Anything, "SKU-GOOD").Return(skuGood, nil)
	mockSKU.On("FindBySKU", mock.Anything, "SKU-DAMAGED").Return(skuDamaged, nil)

	// SKU-GOOD: stock exists, is updated by +2, movement created
	mockStock.On("GetBySKUIDForUpdate", mock.Anything, uint(1), uint(1)).Return(&domainStock.Stock{Quantity: 10}, nil)
	mockStock.On("UpdateQuantity", mock.Anything, uint(1), uint(1), 2).Return(&domainStock.Stock{Quantity: 12}, nil)
	mockStock.On("CreateMovement", mock.Anything, mock.MatchedBy(func(m *domainStock.StockMovement) bool {
		return m.SKUCode == "SKU-GOOD" && m.Type == domainStock.MovementIn && m.Quantity == 2
	})).Return(nil)

	// SKU-DAMAGED: condition DAMAGED -> restock forced to false -> quantity added is 0, movement type is return_damaged
	mockStock.On("GetBySKUID", mock.Anything, uint(2), uint(1)).Return(&domainStock.Stock{Quantity: 5}, nil)
	mockStock.On("CreateMovement", mock.Anything, mock.MatchedBy(func(m *domainStock.StockMovement) bool {
		return m.SKUCode == "SKU-DAMAGED" && m.Quantity == 0 && m.ReferenceType == "return_damaged"
	})).Return(nil)

	completed, err := uc.Complete(context.Background(), returnID, usecaseReturn.CompleteReturnInput{
		CompletedBy: "WarehouseOfficer",
	})

	assert.NoError(t, err)
	assert.NotNil(t, completed)
	assert.Equal(t, domainReturn.StatusCompleted, completed.Status)
	assert.Equal(t, "WarehouseOfficer", completed.CompletedBy)
}
