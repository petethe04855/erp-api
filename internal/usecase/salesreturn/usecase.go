package salesreturn

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainFormula "chawy-erp-api/internal/domain/formula"
	domainInvoice "chawy-erp-api/internal/domain/invoice"
	domainOrder "chawy-erp-api/internal/domain/order"
	domainReturn "chawy-erp-api/internal/domain/salesreturn"
	domainSeq "chawy-erp-api/internal/domain/sequence"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	usecaseFinance "chawy-erp-api/internal/usecase/finance"
	usecaseSeq "chawy-erp-api/internal/usecase/sequence"
	"chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"

	"gorm.io/gorm"
)


type CreateReturnLineInput struct {
	SKU        string                    `json:"sku"`
	Quantity   int                       `json:"quantity"`
	Condition  domainReturn.ItemCondition `json:"condition"`
	Restock    *bool                     `json:"restock"`
	ReasonCode domainReturn.ReasonCode   `json:"reason_code"`
	LotRef     string                    `json:"lot_ref"`
}

type CreateReturnInput struct {
	ReturnType  domainReturn.ReturnType `json:"return_type"`
	OrderID     *uint                   `json:"order_id"`
	WarehouseID uint                    `json:"warehouse_id"`
	ReturnDate  *time.Time              `json:"return_date"`
	Reason      string                  `json:"reason"`
	Note        string                  `json:"note"`
	CreatedBy   string                  `json:"created_by"`
	Lines       []CreateReturnLineInput `json:"lines"`
}

type UpdateReturnInput struct {
	WarehouseID uint                    `json:"warehouse_id"`
	ReturnDate  *time.Time              `json:"return_date"`
	Reason      string                  `json:"reason"`
	Note        string                  `json:"note"`
	Lines       []CreateReturnLineInput `json:"lines"`
}

type CompleteReturnLineInput struct {
	LineID    uint                       `json:"line_id"`
	Condition domainReturn.ItemCondition `json:"condition"`
	Restock   bool                       `json:"restock"`
}

type CompleteReturnInput struct {
	Lines       []CompleteReturnLineInput `json:"lines"`
	CompletedBy string                    `json:"completed_by"`
}

type Usecase interface {
	Create(ctx context.Context, in CreateReturnInput) (*domainReturn.SalesReturn, error)
	GetByID(ctx context.Context, id uint) (*domainReturn.SalesReturn, error)
	List(ctx context.Context, query domainReturn.Query) ([]domainReturn.SalesReturn, int64, error)
	Update(ctx context.Context, id uint, in UpdateReturnInput) (*domainReturn.SalesReturn, error)
	Submit(ctx context.Context, id uint) (*domainReturn.SalesReturn, error)
	Approve(ctx context.Context, id uint, approver string) (*domainReturn.SalesReturn, error)
	Reject(ctx context.Context, id uint, reason string, actor string) (*domainReturn.SalesReturn, error)
	Complete(ctx context.Context, id uint, in CompleteReturnInput) (*domainReturn.SalesReturn, error)
	Cancel(ctx context.Context, id uint, reason string, actor string) (*domainReturn.SalesReturn, error)
	GetOrderReturnable(ctx context.Context, orderID uint) ([]domainReturn.ReturnableItem, error)
}

type salesReturnUsecase struct {
	db          *gorm.DB
	returnRepo  domainReturn.Repository
	orderRepo   domainOrder.Repository
	invoiceRepo domainInvoice.Repository
	skuRepo     domainSKU.Repository
	stockRepo   domainStock.Repository
	formulaRepo domainFormula.Repository
	financeUC   usecaseFinance.Usecase
	seqUsecase  usecaseSeq.Usecase
}

func NewSalesReturnUsecase(
	db *gorm.DB,
	returnRepo domainReturn.Repository,
	orderRepo domainOrder.Repository,
	invoiceRepo domainInvoice.Repository,
	skuRepo domainSKU.Repository,
	stockRepo domainStock.Repository,
	formulaRepo domainFormula.Repository,
	financeUC usecaseFinance.Usecase,
	seqUsecase ...usecaseSeq.Usecase,
) Usecase {
	var su usecaseSeq.Usecase
	if len(seqUsecase) > 0 {
		su = seqUsecase[0]
	}
	return &salesReturnUsecase{
		db:          db,
		returnRepo:  returnRepo,
		orderRepo:   orderRepo,
		invoiceRepo: invoiceRepo,
		skuRepo:     skuRepo,
		stockRepo:   stockRepo,
		formulaRepo: formulaRepo,
		financeUC:   financeUC,
		seqUsecase:  su,
	}
}


func (u *salesReturnUsecase) withTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	if u.db != nil {
		return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return fn(database.WithTxContext(ctx, tx))
		})
	}
	return fn(ctx)
}

func (u *salesReturnUsecase) GetOrderReturnable(ctx context.Context, orderID uint) ([]domainReturn.ReturnableItem, error) {
	order, err := u.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, appErrors.ErrNotFound
	}
	if order.Status != domainOrder.StatusShipped {
		return nil, appErrors.NewAppError("ORDER_NOT_SHIPPED", fmt.Sprintf("Order %s has status %s; returns are only allowed for SHIPPED orders", order.OrderNo, order.Status), 400)
	}

	returnedMap, err := u.returnRepo.CountReturnedQtyByOrderAndSKU(ctx, orderID, 0)
	if err != nil {
		return nil, err
	}

	var items []domainReturn.ReturnableItem
	for _, ol := range order.Items {
		cleanSKU := strings.ToUpper(strings.TrimSpace(ol.SKU))
		returnedSoFar := returnedMap[cleanSKU]
		returnable := ol.Quantity - returnedSoFar
		if returnable < 0 {
			returnable = 0
		}

		items = append(items, domainReturn.ReturnableItem{
			SKU:              ol.SKU,
			Name:             ol.Name,
			OrderedQty:       ol.Quantity,
			ReturnedQtySoFar: returnedSoFar,
			ReturnableQty:    returnable,
			UnitPrice:        ol.Price,
		})
	}

	return items, nil
}

func (u *salesReturnUsecase) Create(ctx context.Context, in CreateReturnInput) (*domainReturn.SalesReturn, error) {
	if in.ReturnType == "" {
		in.ReturnType = domainReturn.ReturnTypeCustomer
	}

	if len(in.Lines) == 0 {
		return nil, appErrors.NewAppError("EMPTY_RETURN", "Return must contain at least one line item", 400)
	}

	if in.WarehouseID == 0 {
		in.WarehouseID = 1
	}

	returnDate := time.Now()
	if in.ReturnDate != nil && !in.ReturnDate.IsZero() {
		returnDate = *in.ReturnDate
	}

	var order *domainOrder.Order
	var invoice *domainInvoice.Invoice
	var returnedMap map[string]int

	if in.ReturnType == domainReturn.ReturnTypeCustomer {
		if in.OrderID == nil || *in.OrderID == 0 {
			return nil, appErrors.NewAppError("ORDER_REQUIRED", "Order reference is required for CUSTOMER returns", 400)
		}

		var err error
		order, err = u.orderRepo.FindByID(ctx, *in.OrderID)
		if err != nil {
			return nil, err
		}
		if order == nil {
			return nil, appErrors.ErrNotFound
		}
		if order.Status != domainOrder.StatusShipped {
			return nil, appErrors.NewAppError("ORDER_NOT_SHIPPED", fmt.Sprintf("Order %s must be in SHIPPED status to create a return", order.OrderNo), 400)
		}

		if u.invoiceRepo != nil {
			invoice, _ = u.invoiceRepo.FindByOrderID(ctx, order.ID)
		}

		returnedMap, err = u.returnRepo.CountReturnedQtyByOrderAndSKU(ctx, order.ID, 0)
		if err != nil {
			return nil, err
		}
	}

	// Prepare Order line price map and orderedQty map
	orderPriceMap := make(map[string]float64)
	orderQtyMap := make(map[string]int)
	orderNameMap := make(map[string]string)
	if order != nil {
		for _, ol := range order.Items {
			key := strings.ToUpper(strings.TrimSpace(ol.SKU))
			orderPriceMap[key] = ol.Price
			orderQtyMap[key] = ol.Quantity
			orderNameMap[key] = ol.Name
		}
	}

	returnNo := fmt.Sprintf("RT-%s-%04d", time.Now().Format("2006/01/02"), time.Now().UnixNano()%10000)
	if u.seqUsecase != nil {
		if genNo, err := u.seqUsecase.Generate(ctx, string(domainSeq.TypeSalesReturn), &returnDate); err == nil && genNo != "" {
			returnNo = genNo
		}
	}

	var subtotal float64
	var totalQty int
	var lines []domainReturn.SalesReturnLine


	for _, l := range in.Lines {
		if l.Quantity <= 0 {
			return nil, appErrors.NewAppError("INVALID_QUANTITY", fmt.Sprintf("Quantity for %s must be greater than 0", l.SKU), 400)
		}

		cleanSKU := strings.ToUpper(strings.TrimSpace(l.SKU))
		var unitPrice float64
		var orderedQty int
		var itemName string

		if in.ReturnType == domainReturn.ReturnTypeCustomer {
			oQty, ok := orderQtyMap[cleanSKU]
			if !ok {
				return nil, appErrors.NewAppError("SKU_NOT_IN_ORDER", fmt.Sprintf("Item %s was not part of order %s", l.SKU, order.OrderNo), 400)
			}
			orderedQty = oQty
			unitPrice = orderPriceMap[cleanSKU]
			itemName = orderNameMap[cleanSKU]

			returnedSoFar := returnedMap[cleanSKU]
			returnable := orderedQty - returnedSoFar
			if l.Quantity > returnable {
				return nil, appErrors.NewAppError("RETURN_QTY_EXCEEDS", fmt.Sprintf("Return quantity %d for %s exceeds available returnable quantity (%d)", l.Quantity, l.SKU, returnable), 400)
			}
		} else {
			// INTERNAL return
			skuEntity, err := u.skuRepo.FindBySKU(ctx, cleanSKU)
			if err == nil && skuEntity != nil {
				unitPrice = skuEntity.Price
				itemName = skuEntity.Name
			} else {
				itemName = l.SKU
			}
		}

		cond := l.Condition
		if cond == "" {
			cond = domainReturn.ConditionGood
		}

		// Enforce business rule: if condition != GOOD, restock is forced to false
		restock := true
		if l.Restock != nil {
			restock = *l.Restock
		}
		if cond != domainReturn.ConditionGood {
			restock = false
		}

		reasonCode := l.ReasonCode
		if reasonCode == "" {
			reasonCode = domainReturn.ReasonCustomerChange
		}

		lineAmount := float64(l.Quantity) * unitPrice
		subtotal += lineAmount
		totalQty += l.Quantity

		lines = append(lines, domainReturn.SalesReturnLine{
			SKU:        cleanSKU,
			Name:       itemName,
			OrderedQty: orderedQty,
			Quantity:   l.Quantity,
			UnitPrice:  unitPrice,
			LineAmount: lineAmount,
			Condition:  cond,
			Restock:    restock,
			ReasonCode: reasonCode,
			LotRef:     l.LotRef,
		})
	}

	netAmount := subtotal // In MVP netAmount mirrors subtotal (with in-vat inclusive)

	salesRet := &domainReturn.SalesReturn{
		ReturnNo:           returnNo,
		ReturnType:         in.ReturnType,
		WarehouseID:        in.WarehouseID,
		ReturnDate:         returnDate,
		Status:             domainReturn.StatusDraft,
		Reason:             in.Reason,
		Note:               in.Note,
		TotalQty:           totalQty,
		Subtotal:           subtotal,
		NetAmount:          netAmount,
		CreatedBy:          in.CreatedBy,
		Lines:              lines,
	}

	if order != nil {
		salesRet.OrderID = &order.ID
		salesRet.OrderNo = order.OrderNo
		salesRet.CustomerID = &order.CustomerID
		salesRet.CustomerName = order.CustomerName
		salesRet.Channel = order.Channel
	}
	if invoice != nil {
		salesRet.InvoiceID = &invoice.ID
		salesRet.InvoiceNo = invoice.InvoiceNo
	}

	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		return u.returnRepo.Create(txCtx, salesRet)
	})
	if err != nil {
		return nil, err
	}

	return salesRet, nil
}

func (u *salesReturnUsecase) GetByID(ctx context.Context, id uint) (*domainReturn.SalesReturn, error) {
	ret, err := u.returnRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return nil, appErrors.ErrNotFound
	}
	return ret, nil
}

func (u *salesReturnUsecase) List(ctx context.Context, query domainReturn.Query) ([]domainReturn.SalesReturn, int64, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	return u.returnRepo.FindAll(ctx, query)
}

func (u *salesReturnUsecase) Update(ctx context.Context, id uint, in UpdateReturnInput) (*domainReturn.SalesReturn, error) {
	var updated *domainReturn.SalesReturn

	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		ret, err := u.returnRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if ret == nil {
			return appErrors.ErrNotFound
		}
		if ret.Status != domainReturn.StatusDraft {
			return appErrors.NewAppError("RETURN_NOT_EDITABLE", "Only DRAFT returns can be edited", 400)
		}

		if len(in.Lines) == 0 {
			return appErrors.NewAppError("EMPTY_RETURN", "Return must contain at least one line item", 400)
		}

		if in.WarehouseID > 0 {
			ret.WarehouseID = in.WarehouseID
		}
		if in.ReturnDate != nil && !in.ReturnDate.IsZero() {
			ret.ReturnDate = *in.ReturnDate
		}
		ret.Reason = in.Reason
		ret.Note = in.Note

		var returnedMap map[string]int
		orderQtyMap := make(map[string]int)
		orderPriceMap := make(map[string]float64)
		orderNameMap := make(map[string]string)

		if ret.OrderID != nil && *ret.OrderID > 0 {
			order, err := u.orderRepo.FindByID(txCtx, *ret.OrderID)
			if err != nil {
				return err
			}
			if order != nil {
				for _, ol := range order.Items {
					k := strings.ToUpper(strings.TrimSpace(ol.SKU))
					orderQtyMap[k] = ol.Quantity
					orderPriceMap[k] = ol.Price
					orderNameMap[k] = ol.Name
				}
			}
			returnedMap, err = u.returnRepo.CountReturnedQtyByOrderAndSKU(txCtx, *ret.OrderID, ret.ID)
			if err != nil {
				return err
			}
		}

		var subtotal float64
		var totalQty int
		var newLines []domainReturn.SalesReturnLine

		for _, l := range in.Lines {
			if l.Quantity <= 0 {
				return appErrors.NewAppError("INVALID_QUANTITY", fmt.Sprintf("Quantity for %s must be greater than 0", l.SKU), 400)
			}
			cleanSKU := strings.ToUpper(strings.TrimSpace(l.SKU))
			var unitPrice float64
			var orderedQty int
			var itemName string

			if ret.OrderID != nil && *ret.OrderID > 0 {
				oQty, ok := orderQtyMap[cleanSKU]
				if !ok {
					return appErrors.NewAppError("SKU_NOT_IN_ORDER", fmt.Sprintf("Item %s was not part of order %s", l.SKU, ret.OrderNo), 400)
				}
				orderedQty = oQty
				unitPrice = orderPriceMap[cleanSKU]
				itemName = orderNameMap[cleanSKU]

				returnedSoFar := returnedMap[cleanSKU]
				returnable := orderedQty - returnedSoFar
				if l.Quantity > returnable {
					return appErrors.NewAppError("RETURN_QTY_EXCEEDS", fmt.Sprintf("Return quantity %d for %s exceeds available returnable quantity (%d)", l.Quantity, l.SKU, returnable), 400)
				}
			} else {
				skuEntity, err := u.skuRepo.FindBySKU(txCtx, cleanSKU)
				if err == nil && skuEntity != nil {
					unitPrice = skuEntity.Price
					itemName = skuEntity.Name
				} else {
					itemName = l.SKU
				}
			}

			cond := l.Condition
			if cond == "" {
				cond = domainReturn.ConditionGood
			}
			restock := true
			if l.Restock != nil {
				restock = *l.Restock
			}
			if cond != domainReturn.ConditionGood {
				restock = false
			}

			lineAmount := float64(l.Quantity) * unitPrice
			subtotal += lineAmount
			totalQty += l.Quantity

			newLines = append(newLines, domainReturn.SalesReturnLine{
				ReturnID:   ret.ID,
				SKU:        cleanSKU,
				Name:       itemName,
				OrderedQty: orderedQty,
				Quantity:   l.Quantity,
				UnitPrice:  unitPrice,
				LineAmount: lineAmount,
				Condition:  cond,
				Restock:    restock,
				ReasonCode: l.ReasonCode,
				LotRef:     l.LotRef,
			})
		}

		ret.Subtotal = subtotal
		ret.NetAmount = subtotal
		ret.TotalQty = totalQty

		if err := u.returnRepo.DeleteLines(txCtx, ret.ID); err != nil {
			return err
		}
		if err := u.returnRepo.CreateLines(txCtx, newLines); err != nil {
			return err
		}
		if err := u.returnRepo.Update(txCtx, ret); err != nil {
			return err
		}

		ret.Lines = newLines
		updated = ret
		return nil
	})

	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *salesReturnUsecase) Submit(ctx context.Context, id uint) (*domainReturn.SalesReturn, error) {
	var updated *domainReturn.SalesReturn
	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		ret, err := u.returnRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if ret == nil {
			return appErrors.ErrNotFound
		}
		if !domainReturn.CanTransition(ret.Status, domainReturn.StatusSubmitted) {
			return appErrors.NewAppError("RETURN_INVALID_TRANSITION", fmt.Sprintf("Cannot transition return from %s to SUBMITTED", ret.Status), 400)
		}

		ret.Status = domainReturn.StatusSubmitted
		if err := u.returnRepo.Update(txCtx, ret); err != nil {
			return err
		}
		updated = ret
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *salesReturnUsecase) Approve(ctx context.Context, id uint, approver string) (*domainReturn.SalesReturn, error) {
	var updated *domainReturn.SalesReturn
	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		ret, err := u.returnRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if ret == nil {
			return appErrors.ErrNotFound
		}
		if !domainReturn.CanTransition(ret.Status, domainReturn.StatusApproved) {
			return appErrors.NewAppError("RETURN_INVALID_TRANSITION", fmt.Sprintf("Cannot transition return from %s to APPROVED", ret.Status), 400)
		}

		// Re-check returnable capacity upon approval
		if ret.OrderID != nil && *ret.OrderID > 0 {
			returnedMap, err := u.returnRepo.CountReturnedQtyByOrderAndSKU(txCtx, *ret.OrderID, ret.ID)
			if err != nil {
				return err
			}
			for _, l := range ret.Lines {
				cleanSKU := strings.ToUpper(strings.TrimSpace(l.SKU))
				returnedSoFar := returnedMap[cleanSKU]
				returnable := l.OrderedQty - returnedSoFar
				if l.Quantity > returnable {
					return appErrors.NewAppError("RETURN_QTY_EXCEEDS", fmt.Sprintf("Return quantity %d for %s exceeds available returnable quantity (%d)", l.Quantity, l.SKU, returnable), 400)
				}
			}
		}

		ret.Status = domainReturn.StatusApproved
		ret.ApprovedBy = approver
		if err := u.returnRepo.Update(txCtx, ret); err != nil {
			return err
		}
		updated = ret
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *salesReturnUsecase) Reject(ctx context.Context, id uint, reason string, actor string) (*domainReturn.SalesReturn, error) {
	var updated *domainReturn.SalesReturn
	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		ret, err := u.returnRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if ret == nil {
			return appErrors.ErrNotFound
		}
		if !domainReturn.CanTransition(ret.Status, domainReturn.StatusRejected) {
			return appErrors.NewAppError("RETURN_INVALID_TRANSITION", fmt.Sprintf("Cannot transition return from %s to REJECTED", ret.Status), 400)
		}

		ret.Status = domainReturn.StatusRejected
		ret.CancellationReason = reason
		ret.ApprovedBy = actor
		if err := u.returnRepo.Update(txCtx, ret); err != nil {
			return err
		}
		updated = ret
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *salesReturnUsecase) Cancel(ctx context.Context, id uint, reason string, actor string) (*domainReturn.SalesReturn, error) {
	var updated *domainReturn.SalesReturn
	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		ret, err := u.returnRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if ret == nil {
			return appErrors.ErrNotFound
		}
		if !domainReturn.CanTransition(ret.Status, domainReturn.StatusCancelled) {
			return appErrors.NewAppError("RETURN_INVALID_TRANSITION", fmt.Sprintf("Cannot cancel return in status %s", ret.Status), 400)
		}

		ret.Status = domainReturn.StatusCancelled
		ret.CancellationReason = reason
		if err := u.returnRepo.Update(txCtx, ret); err != nil {
			return err
		}
		updated = ret
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *salesReturnUsecase) Complete(ctx context.Context, id uint, in CompleteReturnInput) (*domainReturn.SalesReturn, error) {
	var completedRet *domainReturn.SalesReturn

	err := u.withTransaction(ctx, func(txCtx context.Context) error {
		ret, err := u.returnRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if ret == nil {
			return appErrors.ErrNotFound
		}
		if ret.Status == domainReturn.StatusCompleted {
			return appErrors.NewAppError("RETURN_ALREADY_COMPLETED", "Return document is already completed", 400)
		}
		if !domainReturn.CanTransition(ret.Status, domainReturn.StatusCompleted) {
			return appErrors.NewAppError("RETURN_INVALID_TRANSITION", fmt.Sprintf("Cannot complete return from status %s", ret.Status), 400)
		}

		// Apply condition & restock overrides from inspection input
		overrideMap := make(map[uint]CompleteReturnLineInput)
		for _, inp := range in.Lines {
			overrideMap[inp.LineID] = inp
		}

		for i := range ret.Lines {
			line := &ret.Lines[i]
			if ov, ok := overrideMap[line.ID]; ok {
				line.Condition = ov.Condition
				line.Restock = ov.Restock
			}
			// Enforce rule: condition != GOOD cannot restock
			if line.Condition != domainReturn.ConditionGood {
				line.Restock = false
			}
		}

		totalReturnCost := 0.0

		// Process stock replenishment for restockable lines
		for _, line := range ret.Lines {
			cleanSKU := strings.ToUpper(strings.TrimSpace(line.SKU))
			skuEntity, err := u.skuRepo.FindBySKU(txCtx, cleanSKU)
			if err != nil || skuEntity == nil {
				// Might be virtual formula; check if components can be restocked
				formula, fErr := u.formulaRepo.FindByCode(txCtx, cleanSKU)
				if fErr == nil && formula != nil && len(formula.Items) > 0 {
					for _, fi := range formula.Items {
						compSKU, cErr := u.skuRepo.FindBySKU(txCtx, fi.ComponentSKU)
						if cErr == nil && compSKU != nil {
							compQty := fi.Qty * line.Quantity
							if line.Restock {
								stk, _ := u.stockRepo.GetBySKUIDForUpdate(txCtx, compSKU.ID, ret.WarehouseID)
								beforeQty := 0
								if stk != nil {
									beforeQty = stk.Quantity
								}
								updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, compSKU.ID, ret.WarehouseID, compQty)
								if err != nil {
									return err
								}
								_ = u.stockRepo.CreateMovement(txCtx, &domainStock.StockMovement{
									SKUID:         compSKU.ID,
									SKUCode:       compSKU.SKU,
									WarehouseID:   ret.WarehouseID,
									Type:          domainStock.MovementIn,
									Quantity:      compQty,
									BeforeQty:     beforeQty,
									AfterQty:      updatedStk.Quantity,
									ReferenceType: "return",
									ReferenceID:   ret.ReturnNo,
									Note:          fmt.Sprintf("รับคืนชุดสินค้า %s เข้าคลัง (สภาพ: %s)", cleanSKU, line.Condition),
								})
								if compSKU.CostPrice > 0 {
									totalReturnCost += compSKU.CostPrice * float64(compQty)
								}
							}
						}
					}
					continue
				}
				continue
			}

			if line.Restock {
				stk, err := u.stockRepo.GetBySKUIDForUpdate(txCtx, skuEntity.ID, ret.WarehouseID)
				if err != nil {
					return err
				}
				beforeQty := 0
				if stk != nil {
					beforeQty = stk.Quantity
				}

				updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, skuEntity.ID, ret.WarehouseID, line.Quantity)
				if err != nil {
					return err
				}

				_ = u.stockRepo.CreateMovement(txCtx, &domainStock.StockMovement{
					SKUID:         skuEntity.ID,
					SKUCode:       skuEntity.SKU,
					WarehouseID:   ret.WarehouseID,
					Type:          domainStock.MovementIn,
					Quantity:      line.Quantity,
					BeforeQty:     beforeQty,
					AfterQty:      updatedStk.Quantity,
					ReferenceType: "return",
					ReferenceID:   ret.ReturnNo,
					Note:          fmt.Sprintf("รับคืนสินค้าสภาพดีเข้าคลัง (Restock: %s)", line.Condition),
				})

				if skuEntity.CostPrice > 0 {
					totalReturnCost += skuEntity.CostPrice * float64(line.Quantity)
				}
			} else {
				// Record non-restock inbound audit movement
				stk, _ := u.stockRepo.GetBySKUID(txCtx, skuEntity.ID, ret.WarehouseID)
				currQty := 0
				if stk != nil {
					currQty = stk.Quantity
				}
				_ = u.stockRepo.CreateMovement(txCtx, &domainStock.StockMovement{
					SKUID:         skuEntity.ID,
					SKUCode:       skuEntity.SKU,
					WarehouseID:   ret.WarehouseID,
					Type:          domainStock.MovementIn,
					Quantity:      0, // not added to available
					BeforeQty:     currQty,
					AfterQty:      currQty,
					ReferenceType: "return_damaged",
					ReferenceID:   ret.ReturnNo,
					Note:          fmt.Sprintf("รับคืนสินค้าไม่นำกลับเข้าขาย (สภาพ: %s, จำนวน: %d)", line.Condition, line.Quantity),
				})
			}
		}

		// Update return status to COMPLETED
		ret.Status = domainReturn.StatusCompleted
		ret.CompletedBy = in.CompletedBy
		if err := u.returnRepo.Update(txCtx, ret); err != nil {
			return err
		}
		// Save lines condition/restock state INSIDE the same transaction. Must go
		// through the repository (GetDBFromContext resolves the tx from ctx);
		// writing via u.db directly would run on a separate pooled connection,
		// block on row locks held by this very transaction, and hang the request.
		if err := u.returnRepo.UpdateLines(txCtx, ret.Lines); err != nil {
			return err
		}

		// Post Accounting Journal Entries (idempotent, source: sales_return)
		if u.financeUC != nil {
			// Journal 1: Inventory Restock / COGS Reduction (Dr 1300 Inventory / Cr 5000 COGS)
			if totalReturnCost > 0 {
				_, _ = u.financeUC.PostJournal(txCtx, usecaseFinance.PostingRequest{
					SourceType:  "sales_return_stock",
					SourceID:    ret.ID,
					SourceRef:   ret.ReturnNo,
					Description: fmt.Sprintf("รับคืนสินค้าเข้าสต็อกตามใบรับคืน %s", ret.ReturnNo),
					Lines: []usecaseFinance.PostingLineInput{
						{
							AccountCode: usecaseFinance.AccountInventory,
							Debit:       totalReturnCost,
							Credit:      0,
						},
						{
							AccountCode: usecaseFinance.AccountCOGS,
							Debit:       0,
							Credit:      totalReturnCost,
						},
					},
				})
			}

			// Journal 2: Sales Return / AR or Cash Reduction (Dr 5100 Sales Return / Cr 1200 AR)
			if ret.NetAmount > 0 {
				_, _ = u.financeUC.PostJournal(txCtx, usecaseFinance.PostingRequest{
					SourceType:  "sales_return",
					SourceID:    ret.ID,
					SourceRef:   ret.ReturnNo,
					Description: fmt.Sprintf("รับคืนสินค้าและลดยอดลูกหนี้/รายได้ตามใบรับคืน %s", ret.ReturnNo),
					Lines: []usecaseFinance.PostingLineInput{
						{
							AccountCode: usecaseFinance.AccountSalesReturn,
							Debit:       ret.NetAmount,
							Credit:      0,
							Channel:     ret.Channel,
						},
						{
							AccountCode: usecaseFinance.AccountAR,
							Debit:       0,
							Credit:      ret.NetAmount,
							Channel:     ret.Channel,
						},
					},
				})
			}
		}

		completedRet = ret
		return nil
	})

	if err != nil {
		return nil, err
	}
	return completedRet, nil
}
