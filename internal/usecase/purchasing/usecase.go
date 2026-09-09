package purchasing

import (
	"context"
	"fmt"
	"time"

	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	domainSKU "chawy-erp-api/internal/domain/sku"
	domainStock "chawy-erp-api/internal/domain/stock"
	"chawy-erp-api/pkg/database"
	appErrors "chawy-erp-api/pkg/errors"

	"gorm.io/gorm"
)

type CreateSupplierInput struct {
	Code          string
	Name          string
	ContactPerson string
	Phone         string
	Email         string
	Address       string
}

type CreatePOItemInput struct {
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	UnitCost float64 `json:"unit_cost"`
}

type CreatePOInput struct {
	SupplierID uint                `json:"supplier_id"`
	Note       string              `json:"note"`
	Items      []CreatePOItemInput `json:"items"`
}

type ReceiveGoodsInput struct {
	POID        uint
	WarehouseID uint
}

type Usecase interface {
	// Supplier
	CreateSupplier(ctx context.Context, in CreateSupplierInput) (*domainPurchasing.Supplier, error)
	GetSupplierByID(ctx context.Context, id uint) (*domainPurchasing.Supplier, error)
	ListSuppliers(ctx context.Context, q domainPurchasing.SupplierQuery) ([]domainPurchasing.Supplier, int64, error)

	// Purchase Order
	CreatePO(ctx context.Context, in CreatePOInput) (*domainPurchasing.PurchaseOrder, error)
	GetPOByID(ctx context.Context, id uint) (*domainPurchasing.PurchaseOrder, error)
	ListPOs(ctx context.Context, q domainPurchasing.POQuery) ([]domainPurchasing.PurchaseOrder, int64, error)
	ApprovePO(ctx context.Context, id uint) (*domainPurchasing.PurchaseOrder, error)
	ReceiveGoods(ctx context.Context, in ReceiveGoodsInput) (*domainPurchasing.PurchaseOrder, error)
}

type purchasingUsecase struct {
	db             *gorm.DB
	purchasingRepo domainPurchasing.Repository
	skuRepo        domainSKU.Repository
	stockRepo      domainStock.Repository
}

func NewPurchasingUsecase(
	db *gorm.DB,
	purchasingRepo domainPurchasing.Repository,
	skuRepo domainSKU.Repository,
	stockRepo domainStock.Repository,
) Usecase {
	return &purchasingUsecase{
		db:             db,
		purchasingRepo: purchasingRepo,
		skuRepo:        skuRepo,
		stockRepo:      stockRepo,
	}
}

func (u *purchasingUsecase) CreateSupplier(ctx context.Context, in CreateSupplierInput) (*domainPurchasing.Supplier, error) {
	code := in.Code
	if code == "" {
		code = fmt.Sprintf("SUPP-%d", time.Now().UnixNano()%1000000)
	}

	s := &domainPurchasing.Supplier{
		Code:          code,
		Name:          in.Name,
		ContactPerson: in.ContactPerson,
		Phone:         in.Phone,
		Email:         in.Email,
		Address:       in.Address,
		Status:        "active",
	}

	if err := u.purchasingRepo.CreateSupplier(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (u *purchasingUsecase) GetSupplierByID(ctx context.Context, id uint) (*domainPurchasing.Supplier, error) {
	s, err := u.purchasingRepo.FindSupplierByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, appErrors.ErrNotFound
	}
	return s, nil
}

func (u *purchasingUsecase) ListSuppliers(ctx context.Context, q domainPurchasing.SupplierQuery) ([]domainPurchasing.Supplier, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	return u.purchasingRepo.FindAllSuppliers(ctx, q)
}

func (u *purchasingUsecase) CreatePO(ctx context.Context, in CreatePOInput) (*domainPurchasing.PurchaseOrder, error) {
	supp, err := u.purchasingRepo.FindSupplierByID(ctx, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if supp == nil {
		return nil, fmt.Errorf("supplier not found")
	}

	if len(in.Items) == 0 {
		return nil, fmt.Errorf("purchase order must have at least one item")
	}

	poNo := fmt.Sprintf("PO-%s-%04d", time.Now().Format("20060102"), time.Now().UnixNano()%10000)
	var totalCost float64
	var items []domainPurchasing.POItem

	for _, itm := range in.Items {
		skuItem, err := u.skuRepo.FindBySKU(ctx, itm.SKU)
		if err != nil {
			return nil, err
		}
		if skuItem == nil {
			return nil, appErrors.ErrSKUNotFound
		}

		cost := itm.UnitCost
		if cost <= 0 {
			cost = skuItem.CostPrice
		}

		subtotal := cost * float64(itm.Quantity)
		totalCost += subtotal

		items = append(items, domainPurchasing.POItem{
			SKU:      skuItem.SKU,
			Name:     skuItem.Name,
			UnitCost: cost,
			Quantity: itm.Quantity,
			Subtotal: subtotal,
		})
	}

	po := &domainPurchasing.PurchaseOrder{
		PONo:         poNo,
		SupplierID:   supp.ID,
		SupplierName: supp.Name,
		Status:       domainPurchasing.StatusPending,
		TotalAmount:  totalCost,
		Note:         in.Note,
		Items:        items,
	}

	if err := u.purchasingRepo.CreatePO(ctx, po); err != nil {
		return nil, err
	}

	return po, nil
}

func (u *purchasingUsecase) GetPOByID(ctx context.Context, id uint) (*domainPurchasing.PurchaseOrder, error) {
	po, err := u.purchasingRepo.FindPOByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if po == nil {
		return nil, appErrors.ErrNotFound
	}
	return po, nil
}

func (u *purchasingUsecase) ListPOs(ctx context.Context, q domainPurchasing.POQuery) ([]domainPurchasing.PurchaseOrder, int64, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	return u.purchasingRepo.FindAllPOs(ctx, q)
}

func (u *purchasingUsecase) ApprovePO(ctx context.Context, id uint) (*domainPurchasing.PurchaseOrder, error) {
	po, err := u.purchasingRepo.FindPOByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if po == nil {
		return nil, appErrors.ErrNotFound
	}
	if po.Status != domainPurchasing.StatusPending {
		return nil, fmt.Errorf("PO must be PENDING to approve")
	}

	if err := u.purchasingRepo.UpdatePOStatus(ctx, id, domainPurchasing.StatusApproved); err != nil {
		return nil, err
	}
	po.Status = domainPurchasing.StatusApproved
	return po, nil
}

func (u *purchasingUsecase) ReceiveGoods(ctx context.Context, in ReceiveGoodsInput) (*domainPurchasing.PurchaseOrder, error) {
	if in.WarehouseID == 0 {
		in.WarehouseID = 1
	}

	po, err := u.purchasingRepo.FindPOByID(ctx, in.POID)
	if err != nil {
		return nil, err
	}
	if po == nil {
		return nil, appErrors.ErrNotFound
	}
	if po.Status == domainPurchasing.StatusReceived {
		return nil, fmt.Errorf("PO already received")
	}

	// Transaction: increase inventory stock and record movements
	err = u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := database.WithTxContext(ctx, tx)
		for i, line := range po.Items {
			skuEntity, err := u.skuRepo.FindBySKU(txCtx, line.SKU)
			if err != nil || skuEntity == nil {
				return fmt.Errorf("sku %s not found", line.SKU)
			}

			// Get current stock
			stk, err := u.stockRepo.GetBySKUID(txCtx, skuEntity.ID, in.WarehouseID)
			if err != nil {
				return err
			}
			beforeQty := 0
			if stk != nil {
				beforeQty = stk.Quantity
			}

			// Increase stock
			updatedStk, err := u.stockRepo.UpdateQuantity(txCtx, skuEntity.ID, in.WarehouseID, line.Quantity)
			if err != nil {
				return err
			}

			// Record stock IN movement
			movement := &domainStock.StockMovement{
				SKUID:         skuEntity.ID,
				SKUCode:       skuEntity.SKU,
				WarehouseID:   in.WarehouseID,
				Type:          domainStock.MovementIn,
				Quantity:      line.Quantity,
				BeforeQty:     beforeQty,
				AfterQty:      updatedStk.Quantity,
				ReferenceType: "PO_RECEIVE",
				ReferenceID:   po.PONo,
				Note:          fmt.Sprintf("Goods receipt for PO %s", po.PONo),
			}
			if err := u.stockRepo.CreateMovement(txCtx, movement); err != nil {
				return err
			}

			po.Items[i].ReceivedQty = line.Quantity
		}

		po.Status = domainPurchasing.StatusReceived
		return u.purchasingRepo.UpdatePO(txCtx, po)
	})

	if err != nil {
		return nil, err
	}

	return po, nil
}
