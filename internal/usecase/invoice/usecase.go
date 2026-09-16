package invoice

import (
	"context"
	"fmt"
	"time"

	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainSeq "chawy-erp-api/internal/domain/sequence"
	usecaseSeq "chawy-erp-api/internal/usecase/sequence"
	"chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"
)

type CreateInvoiceFromOrderInput struct {
	OrderID uint
}

type MarkPaidInput struct {
	InvoiceID     uint
	Amount        float64
	PaymentMethod string
}

type Usecase interface {
	CreateFromOrder(ctx context.Context, in CreateInvoiceFromOrderInput) (*domainInvoice.Invoice, error)
	GetByID(ctx context.Context, id uint) (*domainInvoice.Invoice, error)
	List(ctx context.Context, q domainInvoice.Query) ([]domainInvoice.Invoice, int64, error)
	MarkAsPaid(ctx context.Context, in MarkPaidInput) (*domainInvoice.Invoice, error)
}

type invoiceUsecase struct {
	invRepo    domainInvoice.Repository
	orderRepo  domainOrder.Repository
	txMgr      database.TxManager
	seqUsecase usecaseSeq.Usecase
}

func NewInvoiceUsecase(invRepo domainInvoice.Repository, orderRepo domainOrder.Repository, seqUsecase ...usecaseSeq.Usecase) Usecase {
	var su usecaseSeq.Usecase
	if len(seqUsecase) > 0 {
		su = seqUsecase[0]
	}
	return &invoiceUsecase{
		invRepo:    invRepo,
		orderRepo:  orderRepo,
		seqUsecase: su,
	}
}

// NewInvoiceUsecaseWithTx wires a transaction manager so payment reads and
// writes are serialized per invoice row (FULL-18).
func NewInvoiceUsecaseWithTx(invRepo domainInvoice.Repository, orderRepo domainOrder.Repository, txMgr database.TxManager, seqUsecase ...usecaseSeq.Usecase) Usecase {
	var su usecaseSeq.Usecase
	if len(seqUsecase) > 0 {
		su = seqUsecase[0]
	}
	return &invoiceUsecase{
		invRepo:    invRepo,
		orderRepo:  orderRepo,
		txMgr:      txMgr,
		seqUsecase: su,
	}
}

func (u *invoiceUsecase) CreateFromOrder(ctx context.Context, in CreateInvoiceFromOrderInput) (*domainInvoice.Invoice, error) {
	orderItem, err := u.orderRepo.FindByID(ctx, in.OrderID)
	if err != nil {
		return nil, err
	}
	if orderItem == nil {
		return nil, appErrors.ErrNotFound
	}

	// Idempotency: If an invoice already exists for this order, return it
	existing, err := u.invRepo.FindByOrderID(ctx, orderItem.ID)
	if err == nil && existing != nil {
		return existing, nil
	}

	invNo := fmt.Sprintf("INV-%s-%04d", time.Now().Format("2006/01/02"), time.Now().UnixNano()%10000)
	if u.seqUsecase != nil {
		if genNo, err := u.seqUsecase.Generate(ctx, string(domainSeq.TypeInvoice), nil); err == nil && genNo != "" {
			invNo = genNo
		}
	}
	dueDate := time.Now().AddDate(0, 0, 30)

	inv := &domainInvoice.Invoice{
		InvoiceNo:    invNo,
		OrderID:      &orderItem.ID,
		OrderNo:      orderItem.OrderNo,
		CustomerID:   orderItem.CustomerID,
		CustomerName: orderItem.CustomerName,
		Amount:       orderItem.TotalAmount,
		PaidAmount:   0,
		Status:       domainInvoice.StatusUnpaid,
		DueDate:      &dueDate,
	}

	if err := u.invRepo.Create(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (u *invoiceUsecase) GetByID(ctx context.Context, id uint) (*domainInvoice.Invoice, error) {
	inv, err := u.invRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, appErrors.ErrNotFound
	}
	return inv, nil
}

func (u *invoiceUsecase) List(ctx context.Context, q domainInvoice.Query) ([]domainInvoice.Invoice, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	return u.invRepo.FindAll(ctx, q)
}

func (u *invoiceUsecase) MarkAsPaid(ctx context.Context, in MarkPaidInput) (*domainInvoice.Invoice, error) {
	if in.Amount < 0 {
		return nil, appErrors.NewAppError("INVALID_AMOUNT", "Payment amount cannot be negative", 400)
	}

	var result *domainInvoice.Invoice

	run := func(ctx context.Context) error {
		// Lock the invoice row so concurrent payments cannot read the same
		// PaidAmount snapshot and overwrite each other (FULL-18).
		inv, err := u.invRepo.FindByIDForUpdate(ctx, in.InvoiceID)
		if err != nil {
			return err
		}
		if inv == nil {
			return appErrors.ErrNotFound
		}
		if inv.Status == domainInvoice.StatusPaid {
			return appErrors.NewAppError("ALREADY_PAID", "Invoice is already fully paid", 400)
		}

		payAmount := in.Amount
		if payAmount == 0 {
			// If no amount was specified (0), pay the remaining unpaid balance in full
			remaining := inv.Amount - inv.PaidAmount
			if remaining <= 0 {
				remaining = inv.Amount
			}
			payAmount = remaining
		}

		newPaidTotal := inv.PaidAmount + payAmount
		if newPaidTotal >= inv.Amount {
			inv.Status = domainInvoice.StatusPaid
			inv.PaidAmount = inv.Amount
			now := time.Now()
			inv.PaidAt = &now
		} else {
			inv.Status = domainInvoice.StatusPartiallyPaid
			inv.PaidAmount = newPaidTotal
		}

		inv.PaymentMethod = in.PaymentMethod
		if inv.PaymentMethod == "" {
			inv.PaymentMethod = "Bank Transfer"
		}

		if err := u.invRepo.Update(ctx, inv); err != nil {
			return err
		}

		result = inv
		return nil
	}

	if u.txMgr != nil {
		if err := u.txMgr.Transaction(ctx, run); err != nil {
			return nil, err
		}
	} else {
		if err := run(ctx); err != nil {
			return nil, err
		}
	}

	return result, nil
}
