package dto

type CreateSupplierRequest struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	ContactPerson string `json:"contact_person"`
	Phone         string `json:"phone"`
	Email         string `json:"email"`
	Address       string `json:"address"`
}

type CreatePOItemDTO struct {
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	UnitCost float64 `json:"unit_cost"`
}

type CreatePORequest struct {
	SupplierID uint              `json:"supplier_id"`
	Note       string            `json:"note"`
	Items      []CreatePOItemDTO `json:"items"`
}

type ReceiveGoodsRequest struct {
	WarehouseID uint `json:"warehouse_id"`
}
