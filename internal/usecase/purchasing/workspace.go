package purchasing

import (
	"context"
	"fmt"
	"strings"
	"time"

	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainStock "chawy-erp-api/internal/domain/stock"
	appErrors "chawy-erp-api/pkg/errors"
)

// CreatePurchaseOrderItem is one PO line as sent by the workspace PO screen.
type CreatePurchaseOrderItem struct {
	SKU      string
	Name     string
	Quantity int
	Qty      int
	UnitCost float64
}

// CreatePurchaseOrderInput mirrors the workspace purchase-order payload.
type CreatePurchaseOrderInput struct {
	Supplier             string
	SupplierID           uint
	Note                 string
	ETADate              string
	ExpectedDeliveryDays int
	Items                []CreatePurchaseOrderItem
}

// CreatePurchaseOrder creates a PO from the workspace screen, resolving (or
// creating) the supplier by name and computing the ETA on the backend.
func (u *purchasingUsecase) CreatePurchaseOrder(ctx context.Context, in CreatePurchaseOrderInput) (*domainPurchasing.PurchaseOrder, error) {
	if len(in.Items) == 0 {
		return nil, appErrors.NewAppError("PO_ITEMS_REQUIRED", "Purchase order must contain at least one item", 400)
	}

	supplierID, supplierName, err := u.resolveSupplier(ctx, in.SupplierID, in.Supplier)
	if err != nil {
		return nil, err
	}

	poItems := make([]domainPurchasing.POItem, len(in.Items))
	var total float64
	for i, it := range in.Items {
		qty := it.Quantity
		if qty <= 0 && it.Qty > 0 {
			qty = it.Qty
		}
		if qty <= 0 {
			return nil, appErrors.NewAppError("PO_ITEM_QTY_INVALID",
				fmt.Sprintf("Item quantity for SKU %s must be greater than 0", it.SKU), 400)
		}
		if it.UnitCost < 0 {
			return nil, appErrors.NewAppError("PO_ITEM_COST_INVALID",
				fmt.Sprintf("Item unit cost for SKU %s cannot be negative", it.SKU), 400)
		}

		sub := it.UnitCost * float64(qty)
		total += sub
		poItems[i] = domainPurchasing.POItem{
			SKU:       it.SKU,
			Name:      it.Name,
			UnitCost:  it.UnitCost,
			Quantity:  qty,
			Subtotal:  sub,
			CreatedAt: time.Now(),
		}
	}

	note := in.Note
	if in.ETADate != "" && !strings.Contains(note, "ETA:") {
		if note != "" {
			note += " "
		}
		note += fmt.Sprintf("ETA:%s", in.ETADate)
	}

	etaDate := in.ETADate
	if etaDate == "" {
		// ETA computation stays on the backend: clients may send an explicit
		// etaDate or a lead-time in days, never a pre-computed date.
		leadDays := in.ExpectedDeliveryDays
		if leadDays <= 0 {
			leadDays = 7
		}
		etaDate = time.Now().AddDate(0, 0, leadDays).Format("2006-01-02")
	}

	po := &domainPurchasing.PurchaseOrder{
		PONo:         fmt.Sprintf("PO-%s-%04d", time.Now().Format("2006/01/02"), time.Now().Unix()%10000),
		SupplierID:   supplierID,
		SupplierName: supplierName,
		Status:       domainPurchasing.StatusPending,
		TotalAmount:  total,
		Note:         note,
		ETADate:      etaDate,
		Items:        poItems,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := u.purchasingRepo.CreatePO(ctx, po); err != nil {
		return nil, err
	}
	return po, nil
}

// resolveSupplier finds a supplier by id or name, creating a new supplier
// record when only a free-text name is supplied.
func (u *purchasingUsecase) resolveSupplier(ctx context.Context, supplierID uint, supplierName string) (uint, string, error) {
	if supplierID > 0 {
		sup, err := u.purchasingRepo.FindSupplierByID(ctx, supplierID)
		if err != nil {
			return 0, "", err
		}
		if sup != nil {
			return sup.ID, sup.Name, nil
		}
		return supplierID, "", nil
	}

	name := strings.TrimSpace(supplierName)
	if name == "" {
		return 0, "", nil
	}

	existing, _, err := u.purchasingRepo.FindAllSuppliers(ctx, domainPurchasing.SupplierQuery{Search: name, Page: 1, Limit: 20})
	if err == nil {
		for i := range existing {
			if strings.EqualFold(existing[i].Name, name) {
				return existing[i].ID, existing[i].Name, nil
			}
		}
	}

	sup := &domainPurchasing.Supplier{
		Code:      fmt.Sprintf("SUP-%s", time.Now().Format("20060102150405")),
		Name:      name,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := u.purchasingRepo.CreateSupplier(ctx, sup); err != nil {
		return 0, name, nil
	}
	return sup.ID, sup.Name, nil
}

// UpdatePOStatus sets a purchase order's status.
func (u *purchasingUsecase) UpdatePOStatus(ctx context.Context, id uint, status string) (*domainPurchasing.PurchaseOrder, error) {
	po, err := u.purchasingRepo.FindPOByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if po == nil {
		return nil, appErrors.NewAppError("PO_NOT_FOUND", "Purchase order not found", 404)
	}

	po.Status = domainPurchasing.POStatus(strings.ToUpper(status))
	po.UpdatedAt = time.Now()
	if err := u.purchasingRepo.UpdatePO(ctx, po); err != nil {
		return nil, appErrors.NewAppError("PO_UPDATE_FAILED", "Failed to update purchase order status", 500)
	}
	return po, nil
}

// ListGoodsReceives returns the workspace goods-receive list.
func (u *purchasingUsecase) ListGoodsReceives(ctx context.Context, q domainPurchasing.GoodsReceiveQuery) ([]domainPurchasing.GoodsReceive, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 50
	}
	return u.purchasingRepo.FindAllGoodsReceives(ctx, q)
}

// GetGoodsReceive returns one goods-receive document with its lines.
func (u *purchasingUsecase) GetGoodsReceive(ctx context.Context, id uint) (*domainPurchasing.GoodsReceive, error) {
	gr, err := u.purchasingRepo.FindGoodsReceiveByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if gr == nil {
		return nil, appErrors.NewAppError("GOODS_RECEIVE_NOT_FOUND", "Goods receive not found", 404)
	}
	return gr, nil
}

// ListGoodsIssues returns the workspace goods-issue history (OUT movements).
func (u *purchasingUsecase) ListGoodsIssues(ctx context.Context, q domainPurchasing.GoodsIssueQuery) ([]domainStock.StockMovement, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 50
	}
	return u.purchasingRepo.FindAllGoodsIssues(ctx, q)
}

// GetGoodsIssue returns one goods-issue record.
func (u *purchasingUsecase) GetGoodsIssue(ctx context.Context, id uint) (*domainStock.StockMovement, error) {
	m, err := u.purchasingRepo.FindGoodsIssueByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, appErrors.NewAppError("GOODS_ISSUE_NOT_FOUND", "Goods issue not found", 404)
	}
	return m, nil
}
