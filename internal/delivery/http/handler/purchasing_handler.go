package handler

import (
	"strconv"

	"chawy-erp-api/internal/delivery/http/dto"
	domainPurchasing "chawy-erp-api/internal/domain/purchasing"
	usecasePurchasing "chawy-erp-api/internal/usecase/purchasing"
	"chawy-erp-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type PurchasingHandler struct {
	usecase usecasePurchasing.Usecase
}

func NewPurchasingHandler(usecase usecasePurchasing.Usecase) *PurchasingHandler {
	return &PurchasingHandler{usecase: usecase}
}

// Supplier
func (h *PurchasingHandler) CreateSupplier(c *fiber.Ctx) error {
	var req dto.CreateSupplierRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.Name == "" {
		return response.BadRequest(c, "Supplier name is required")
	}

	result, err := h.usecase.CreateSupplier(c.Context(), usecasePurchasing.CreateSupplierInput{
		Code:          req.Code,
		Name:          req.Name,
		ContactPerson: req.ContactPerson,
		Phone:         req.Phone,
		Email:         req.Email,
		Address:       req.Address,
	})
	if err != nil {
		return err
	}

	return response.Created(c, result, "Supplier created successfully")
}

func (h *PurchasingHandler) GetSupplierByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid supplier ID")
	}

	result, err := h.usecase.GetSupplierByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, result)
}

func (h *PurchasingHandler) ListSuppliers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")
	status := c.Query("status", "")

	items, total, err := h.usecase.ListSuppliers(c.Context(), domainPurchasing.SupplierQuery{
		Search: search,
		Status: status,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, items, page, limit, total)
}

// Purchase Order
func (h *PurchasingHandler) CreatePO(c *fiber.Ctx) error {
	var req dto.CreatePORequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	if req.SupplierID == 0 || len(req.Items) == 0 {
		return response.BadRequest(c, "Supplier ID and at least one item are required")
	}

	itemInputs := make([]usecasePurchasing.CreatePOItemInput, len(req.Items))
	for i, itm := range req.Items {
		itemInputs[i] = usecasePurchasing.CreatePOItemInput{
			SKU:      itm.SKU,
			Quantity: itm.Quantity,
			UnitCost: itm.UnitCost,
		}
	}

	result, err := h.usecase.CreatePO(c.Context(), usecasePurchasing.CreatePOInput{
		SupplierID: req.SupplierID,
		Note:       req.Note,
		Items:      itemInputs,
	})
	if err != nil {
		return err
	}

	return response.Created(c, result, "Purchase order created successfully")
}

func (h *PurchasingHandler) GetPOByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid PO ID")
	}

	result, err := h.usecase.GetPOByID(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, result)
}

func (h *PurchasingHandler) ListPOs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	search := c.Query("search", "")
	status := c.Query("status", "")
	supplierID, _ := strconv.ParseUint(c.Query("supplierId", "0"), 10, 32)

	items, total, err := h.usecase.ListPOs(c.Context(), domainPurchasing.POQuery{
		Search:     search,
		SupplierID: uint(supplierID),
		Status:     status,
		Page:       page,
		Limit:      limit,
	})
	if err != nil {
		return err
	}

	return response.List(c, items, page, limit, total)
}

func (h *PurchasingHandler) ApprovePO(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid PO ID")
	}

	result, err := h.usecase.ApprovePO(c.Context(), uint(id))
	if err != nil {
		return err
	}

	return response.OK(c, result, "Purchase order approved successfully")
}

func (h *PurchasingHandler) ReceiveGoods(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid PO ID")
	}

	var req dto.ReceiveGoodsRequest
	_ = c.BodyParser(&req)
	if req.WarehouseID == 0 {
		req.WarehouseID = 1
	}

	result, err := h.usecase.ReceiveGoods(c.Context(), usecasePurchasing.ReceiveGoodsInput{
		POID:        uint(id),
		WarehouseID: req.WarehouseID,
	})
	if err != nil {
		return err
	}

	return response.OK(c, result, "Goods received and stock updated successfully")
}
