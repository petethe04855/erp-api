package quotation

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	domainOrder "chawy-erp-api/internal/domain/order"
	domainQuotation "chawy-erp-api/internal/domain/quotation"
	domainSKU "chawy-erp-api/internal/domain/sku"
	appErrors "chawy-erp-api/pkg/errors"
)

// SKUFinder resolves SKU records during line validation.
type SKUFinder interface {
	FindBySKU(ctx context.Context, skuCode string) (*domainSKU.SKU, error)
}

// OrderCreator persists sales orders (used by quotation conversion).
type OrderCreator interface {
	Create(ctx context.Context, order *domainOrder.Order) error
}

// TxManager coordinates transactions (backed by pkg/database.NewTxManager).
type TxManager interface {
	Transaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

type Usecase interface {
	List(ctx context.Context, page, limit int) ([]domainQuotation.Quotation, int64, error)
	Create(ctx context.Context, input CreateInput) (*domainQuotation.Quotation, error)
	GetByID(ctx context.Context, id uint) (*domainQuotation.Quotation, error)
	UpdateStatus(ctx context.Context, id uint, newStatus domainQuotation.Status) (*domainQuotation.Quotation, error)
	ConvertToSalesOrder(ctx context.Context, id uint) (*ConversionResult, error)
}

type quotationUsecase struct {
	repo      domainQuotation.Repository
	skuRepo   SKUFinder
	orderRepo OrderCreator
	txMgr     TxManager
}

func NewQuotationUsecase(repo domainQuotation.Repository, skuRepo SKUFinder, orderRepo OrderCreator, txMgr TxManager) Usecase {
	return &quotationUsecase{repo: repo, skuRepo: skuRepo, orderRepo: orderRepo, txMgr: txMgr}
}

func (u *quotationUsecase) List(ctx context.Context, page, limit int) ([]domainQuotation.Quotation, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	return u.repo.List(ctx, page, limit)
}

// Create validates client business data server-side and persists a quotation.
// It never trusts client prices/quantities/dates/status blindly (P1 fix:
// quotation API must not trust business data from the client).
func (u *quotationUsecase) Create(ctx context.Context, input CreateInput) (*domainQuotation.Quotation, error) {
	if strings.TrimSpace(input.Customer) == "" {
		return nil, appErrors.NewAppError("QUOTATION_CUSTOMER_REQUIRED", "Customer name is required", 400)
	}
	if len(input.Lines) == 0 {
		return nil, appErrors.NewAppError("QUOTATION_LINES_REQUIRED", "At least one item line is required", 400)
	}

	// Server-side business validation (never trust client business data).
	for i, l := range input.Lines {
		if l.Qty <= 0 {
			return nil, appErrors.NewAppError("QUOTATION_INVALID_QTY", fmt.Sprintf("Line %d: quantity must be positive", i+1), 400)
		}
		if l.Price < 0 {
			return nil, appErrors.NewAppError("QUOTATION_INVALID_PRICE", fmt.Sprintf("Line %d: price must not be negative", i+1), 400)
		}
		if strings.TrimSpace(l.SKU) == "" {
			return nil, appErrors.NewAppError("QUOTATION_SKU_REQUIRED", fmt.Sprintf("Line %d: SKU is required", i+1), 400)
		}
		// Verify the SKU actually exists (and matches the product when given).
		skuCode := strings.ToUpper(strings.TrimSpace(l.SKU))
		product, err := u.skuRepo.FindBySKU(ctx, skuCode)
		if err != nil {
			return nil, err
		}
		if product == nil {
			return nil, appErrors.NewAppError("QUOTATION_SKU_NOT_FOUND", fmt.Sprintf("Line %d: SKU %s not found", i+1, l.SKU), 400)
		}
		if l.ProductID != 0 && l.ProductID != product.ID {
			return nil, appErrors.NewAppError("QUOTATION_PRODUCT_MISMATCH", fmt.Sprintf("Line %d: product does not match SKU %s", i+1, l.SKU), 400)
		}
		input.Lines[i].ProductID = product.ID
		// Price stays client-supplied: quotation pricing is a business offer,
		// but the total is always recomputed below from validated lines.
	}

	// API-TS-02: Quotations MUST always start in Draft status.
	// Reject any attempt to create a quotation in non-Draft status (such as Approved or Converted)
	// to prevent bypassing the domain state machine. Transitions must go through UpdateStatus.
	if input.Status != "" && domainQuotation.Status(input.Status) != domainQuotation.StatusDraft {
		return nil, appErrors.NewAppError("QUOTATION_INVALID_STATUS",
			fmt.Sprintf("New quotations must be created with status '%s'", domainQuotation.StatusDraft), 400)
	}
	status := domainQuotation.StatusDraft

	dateStr := input.Date
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	} else if !domainQuotation.IsValidDate(dateStr) {
		return nil, appErrors.NewAppError("QUOTATION_INVALID_DATE", "Invalid date format, expected YYYY-MM-DD", 400)
	}
	validUntilStr := input.ValidUntil
	if validUntilStr == "" {
		validUntilStr = time.Now().AddDate(0, 0, 15).Format("2006-01-02")
	} else if !domainQuotation.IsValidDate(validUntilStr) {
		return nil, appErrors.NewAppError("QUOTATION_INVALID_VALID_UNTIL", "Invalid validUntil format, expected YYYY-MM-DD", 400)
	}
	if validUntilStr < dateStr {
		return nil, appErrors.NewAppError("QUOTATION_INVALID_RANGE", "validUntil must not be before date", 400)
	}

	var total float64
	lines := make([]domainQuotation.QuotationLine, len(input.Lines))
	for i, l := range input.Lines {
		sub := l.Price * float64(l.Qty)
		total += sub
		lines[i] = domainQuotation.QuotationLine{
			ProductID: l.ProductID,
			SKU:       strings.ToUpper(strings.TrimSpace(l.SKU)),
			Name:      l.Name,
			Price:     l.Price,
			Quantity:  l.Qty,
			Subtotal:  sub,
			CreatedAt: time.Now(),
		}
	}

	// API-TS-03: Generate collision-resistant unique code with nanosecond precision and cryptographic entropy.
	now := time.Now()
	rnd, err := rand.Int(rand.Reader, big.NewInt(100000000))
	var rndVal int64
	if err != nil {
		rndVal = now.UnixNano() % 100000000
	} else {
		rndVal = rnd.Int64()
	}
	code := fmt.Sprintf("QT-%s-%09d-%08d", now.Format("20060102"), now.UnixNano()%1000000000, rndVal)

	q := &domainQuotation.Quotation{
		Code:         code,
		CustomerName: input.Customer,
		Date:         dateStr,
		ValidUntil:   validUntilStr,
		LeadSource:   input.LeadSource,
		Status:       status,
		TotalAmount:  total,
		Note:         input.Note,
		Lines:        lines,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := u.repo.Create(ctx, q); err != nil {
		return nil, fmt.Errorf("failed to create quotation: %w", err)
	}
	return q, nil
}

func (u *quotationUsecase) GetByID(ctx context.Context, id uint) (*domainQuotation.Quotation, error) {
	q, err := u.repo.FindByIDWithLines(ctx, id)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, appErrors.NewAppError("QUOTATION_NOT_FOUND", "Quotation not found", 404)
	}
	return q, nil
}

// UpdateStatus enforces the domain state machine (Draft -> Sent ->
// Approved -> Converted; Rejected terminal).
func (u *quotationUsecase) UpdateStatus(ctx context.Context, id uint, newStatus domainQuotation.Status) (*domainQuotation.Quotation, error) {
	if !domainQuotation.IsValidStatus(newStatus) {
		return nil, appErrors.NewAppError("QUOTATION_INVALID_STATUS", "Invalid quotation status", 400)
	}

	var updated *domainQuotation.Quotation
	err := u.txMgr.Transaction(ctx, func(txCtx context.Context) error {
		q, err := u.repo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if q == nil {
			return appErrors.NewAppError("QUOTATION_NOT_FOUND", "Quotation not found", 404)
		}
		if !domainQuotation.CanTransition(q.Status, newStatus) {
			return appErrors.NewAppError("QUOTATION_INVALID_TRANSITION",
				fmt.Sprintf("Cannot change quotation status from %s to %s", q.Status, newStatus), 400)
		}
		q.Status = newStatus
		q.UpdatedAt = time.Now()
		if err := u.repo.Save(txCtx, q); err != nil {
			return err
		}
		updated = q
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ConvertToSalesOrder converts an Approved, non-expired quotation with valid
// lines into a pending Sales Order inside one transaction, locking the
// quotation row so a concurrent conversion cannot double-create orders.
func (u *quotationUsecase) ConvertToSalesOrder(ctx context.Context, id uint) (*ConversionResult, error) {
	var result *ConversionResult

	err := u.txMgr.Transaction(ctx, func(txCtx context.Context) error {
		q, err := u.repo.FindByIDForUpdateWithLines(txCtx, id)
		if err != nil {
			return err
		}
		if q == nil {
			return appErrors.NewAppError("QUOTATION_NOT_FOUND", "Quotation not found", 404)
		}
		// Business rule: only Approved quotations can be converted (state
		// machine in domain/quotation — Draft/Rejected/Sent cannot convert).
		if q.Status != domainQuotation.StatusApproved {
			return appErrors.NewAppError("QUOTATION_NOT_APPROVED", "Only Approved quotations can be converted to a Sales Order", 400)
		}
		// Block expired quotations.
		if domainQuotation.IsValidDate(q.ValidUntil) {
			if validUntil, _ := time.Parse("2006-01-02", q.ValidUntil); validUntil.Before(time.Now()) {
				return appErrors.NewAppError("QUOTATION_EXPIRED", "Quotation has expired", 400)
			}
		}
		if len(q.Lines) == 0 {
			return appErrors.NewAppError("QUOTATION_NO_LINES", "Quotation has no lines to convert", 400)
		}
		for _, l := range q.Lines {
			if l.Quantity <= 0 || l.Price < 0 || l.SKU == "" {
				return appErrors.NewAppError("QUOTATION_INVALID_LINES", "Quotation contains invalid line data", 400)
			}
		}

		ordItems := make([]domainOrder.OrderItem, len(q.Lines))
		for i, l := range q.Lines {
			ordItems[i] = domainOrder.OrderItem{
				SKU:       l.SKU,
				Name:      l.Name,
				Price:     l.Price,
				Quantity:  l.Quantity,
				Subtotal:  l.Subtotal,
				CreatedAt: time.Now(),
			}
		}

		channel := "direct"
		if q.LeadSource != "" {
			channel = strings.ToLower(q.LeadSource)
		}

		now := time.Now()
		rnd, err := rand.Int(rand.Reader, big.NewInt(100000000))
		var rndVal int64
		if err != nil {
			rndVal = now.UnixNano() % 100000000
		} else {
			rndVal = rnd.Int64()
		}
		orderNo := fmt.Sprintf("SO-%s-%09d-%08d", now.Format("20060102"), now.UnixNano()%1000000000, rndVal)

		ord := &domainOrder.Order{
			OrderNo:      orderNo,
			CustomerName: q.CustomerName,
			Channel:      channel,
			Status:       domainOrder.StatusPending,
			TotalAmount:  q.TotalAmount,
			Note:         fmt.Sprintf("Converted from Quotation %s", q.Code),
			Items:        ordItems,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		// Create the order inside the same transaction: the order repository
		// resolves the tx handle from txCtx (GetDBFromContext).
		if err := u.orderRepo.Create(txCtx, ord); err != nil {
			return fmt.Errorf("failed to create sales order: %w", err)
		}

		q.Status = domainQuotation.StatusConverted
		q.UpdatedAt = time.Now()
		if err := u.repo.Save(txCtx, q); err != nil {
			return err
		}

		result = &ConversionResult{
			QuotationID: q.ID,
			OrderID:     ord.ID,
			OrderNo:     orderNo,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
