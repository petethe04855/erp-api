package dto

import (
	"time"

	domainStockOut "chawy-erp-api/internal/domain/stockout"
)

// StockOutLineRequest is one tapped SKU quantity.
type StockOutLineRequest struct {
	SKUID uint `json:"skuId"`
	Qty   int  `json:"qty"`
}

// StockOutBatchRequest saves one count from a device.
type StockOutBatchRequest struct {
	ClientID  string                `json:"clientId"`
	CreatedAt *time.Time            `json:"createdAt"`
	Lines     []StockOutLineRequest `json:"lines"`
}

// StockOutReviewRequest closes a short batch.
type StockOutReviewRequest struct {
	Note string `json:"note"`
}

// StockOutBoxDTO is one tile in the config.
type StockOutBoxDTO struct {
	SKUID    uint   `json:"skuId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Group    string `json:"group"`
	Unit     string `json:"unit"`
	Color    string `json:"color"`
	Image    string `json:"image"`
	LowStock int    `json:"lowStock"`
}

// StockOutConfigDTO is the config, in requests and responses.
type StockOutConfigDTO struct {
	WarehouseID uint             `json:"warehouseId"`
	Boxes       []StockOutBoxDTO `json:"boxes"`
	CountRoles  []string         `json:"countRoles"`
	VoidRoles   []string         `json:"voidRoles"`
}

// ToDomain converts the request config.
func (c StockOutConfigDTO) ToDomain() domainStockOut.Config {
	out := domainStockOut.Config{WarehouseID: c.WarehouseID, Boxes: []domainStockOut.Box{},
		CountRoles: append([]string{}, c.CountRoles...), VoidRoles: append([]string{}, c.VoidRoles...)}
	for _, b := range c.Boxes {
		out.Boxes = append(out.Boxes, domainStockOut.Box(b))
	}
	return out
}

// StockOutConfigFromDomain converts a stored config.
func StockOutConfigFromDomain(c domainStockOut.Config) StockOutConfigDTO {
	out := StockOutConfigDTO{WarehouseID: c.WarehouseID, Boxes: []StockOutBoxDTO{},
		CountRoles: append([]string{}, c.CountRoles...), VoidRoles: append([]string{}, c.VoidRoles...)}
	for _, b := range c.Boxes {
		out.Boxes = append(out.Boxes, StockOutBoxDTO(b))
	}
	return out
}

// StockOutLotDTO is how much of a line came out of one lot.
type StockOutLotDTO struct {
	LotID     uint   `json:"lotId"`
	LotNumber string `json:"lotNumber"`
	Expiry    string `json:"expiry"`
	Qty       int    `json:"qty"`
}

// StockOutLineDTO is one SKU in a batch.
type StockOutLineDTO struct {
	SKUID    uint             `json:"skuId"`
	SKUCode  string           `json:"skuCode"`
	Name     string           `json:"name"`
	Group    string           `json:"group"`
	Qty      int              `json:"qty"`
	Deducted int              `json:"deducted"`
	Short    int              `json:"short"`
	Lots     []StockOutLotDTO `json:"lots"`
}

// StockOutBatchDTO is a saved count.
type StockOutBatchDTO struct {
	ID          uint              `json:"id"`
	BatchNo     string            `json:"batchNo"`
	ClientID    string            `json:"clientId"`
	WarehouseID uint              `json:"warehouseId"`
	UserID      uint              `json:"userId"`
	UserName    string            `json:"userName"`
	Lines       []StockOutLineDTO `json:"lines"`
	TotalQty    int               `json:"totalQty"`
	Status      string            `json:"status"`
	VoidedAt    *time.Time        `json:"voidedAt"`
	VoidedBy    string            `json:"voidedBy"`
	ReviewedAt  *time.Time        `json:"reviewedAt"`
	ReviewedBy  string            `json:"reviewedBy"`
	ReviewNote  string            `json:"reviewNote"`
	CountedAt   time.Time         `json:"countedAt"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// StockOutBatchFromDomain converts a stored batch.
func StockOutBatchFromDomain(b *domainStockOut.Batch) StockOutBatchDTO {
	lines := make([]StockOutLineDTO, 0, len(b.Lines))
	for _, l := range b.Lines {
		lots := make([]StockOutLotDTO, 0, len(l.Lots))
		for _, t := range l.Lots {
			lots = append(lots, StockOutLotDTO(t))
		}
		lines = append(lines, StockOutLineDTO{SKUID: l.SKUID, SKUCode: l.SKUCode, Name: l.Name, Group: l.Group,
			Qty: l.Qty, Deducted: l.Deducted, Short: l.Short, Lots: lots})
	}
	return StockOutBatchDTO{
		ID: b.ID, BatchNo: b.BatchNo, ClientID: b.ClientID, WarehouseID: b.WarehouseID, UserID: b.UserID,
		UserName: b.UserName, Lines: lines, TotalQty: b.TotalQty, Status: b.Status,
		VoidedAt: b.VoidedAt, VoidedBy: b.VoidedBy, ReviewedAt: b.ReviewedAt, ReviewedBy: b.ReviewedBy,
		ReviewNote: b.ReviewNote, CountedAt: b.CountedAt, CreatedAt: b.CreatedAt,
	}
}
