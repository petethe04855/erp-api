package dto

// CRMGoodsIssueRequest is the body of POST /api/v1/integrations/crm/goods-issues
// (sent by backoffice-api when a reward ships).
type CRMGoodsIssueRequest struct {
	IdempotencyKey string                 `json:"idempotencyKey"`
	WarehouseID    uint                   `json:"warehouseId"`
	Items          []CRMGoodsIssueItemReq `json:"items"`
	Reference      string                 `json:"reference"`
	Note           string                 `json:"note"`
}

// CRMGoodsIssueItemReq is one requested SKU quantity.
type CRMGoodsIssueItemReq struct {
	SKUID uint `json:"skuId"`
	Qty   int  `json:"qty"`
}
