package dto

import (
	"time"

	domainBooth "chawy-erp-api/internal/domain/booth"
)

// BoothCartLine is one SKU quantity in a booth cart.
type BoothCartLine struct {
	SKUID uint `json:"skuId"`
	Qty   int  `json:"qty"`
}

// BoothQuoteRequest prices a cart without saving it.
type BoothQuoteRequest struct {
	Lines []BoothCartLine `json:"lines"`
}

// BoothSaleRequest records a bill and takes its stock out of the inventory.
type BoothSaleRequest struct {
	ClientRef     string          `json:"clientRef"`
	Lines         []BoothCartLine `json:"lines"`
	PaymentMethod string          `json:"paymentMethod"`
	CashReceived  *float64        `json:"cashReceived"`
	ExpectedTotal *float64        `json:"expectedTotal"`
}

// BoothVoidRequest voids a bill.
type BoothVoidRequest struct {
	Reason string `json:"reason"`
}

// BoothFlavorDTO is a treat button priced by the set prices.
type BoothFlavorDTO struct {
	SKUID uint   `json:"skuId"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// BoothSetDTO is one set price.
type BoothSetDTO struct {
	Size  int     `json:"size"`
	Price float64 `json:"price"`
}

// BoothItemDTO is a per-piece button.
type BoothItemDTO struct {
	SKUID uint    `json:"skuId"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

// BoothDiscountDTO is the minimum-spend discount.
type BoothDiscountDTO struct {
	Enabled bool    `json:"enabled"`
	Min     float64 `json:"min"`
	Type    string  `json:"type"`
	Value   float64 `json:"value"`
}

// BoothConfigDTO is the booth config, in requests and responses.
type BoothConfigDTO struct {
	WarehouseID uint             `json:"warehouseId"`
	Flavors     []BoothFlavorDTO `json:"flavors"`
	Sets        []BoothSetDTO    `json:"sets"`
	Items       []BoothItemDTO   `json:"items"`
	Discount    BoothDiscountDTO `json:"discount"`
}

// ToDomain converts the request config.
func (c BoothConfigDTO) ToDomain() domainBooth.Config {
	out := domainBooth.Config{
		WarehouseID: c.WarehouseID,
		Flavors:     []domainBooth.Flavor{}, Sets: []domainBooth.SetPrice{}, Items: []domainBooth.Item{},
		Discount: domainBooth.Discount{Enabled: c.Discount.Enabled, Min: c.Discount.Min, Type: c.Discount.Type, Value: c.Discount.Value},
	}
	for _, f := range c.Flavors {
		out.Flavors = append(out.Flavors, domainBooth.Flavor{SKUID: f.SKUID, Name: f.Name, Color: f.Color})
	}
	for _, s := range c.Sets {
		out.Sets = append(out.Sets, domainBooth.SetPrice{Size: s.Size, Price: s.Price})
	}
	for _, it := range c.Items {
		out.Items = append(out.Items, domainBooth.Item{SKUID: it.SKUID, Name: it.Name, Price: it.Price})
	}
	return out
}

// BoothConfigFromDomain converts a stored config.
func BoothConfigFromDomain(c domainBooth.Config) BoothConfigDTO {
	out := BoothConfigDTO{
		WarehouseID: c.WarehouseID,
		Flavors:     []BoothFlavorDTO{}, Sets: []BoothSetDTO{}, Items: []BoothItemDTO{},
		Discount: BoothDiscountDTO{Enabled: c.Discount.Enabled, Min: c.Discount.Min, Type: c.Discount.Type, Value: c.Discount.Value},
	}
	for _, f := range c.Flavors {
		out.Flavors = append(out.Flavors, BoothFlavorDTO{SKUID: f.SKUID, Name: f.Name, Color: f.Color})
	}
	for _, s := range c.Sets {
		out.Sets = append(out.Sets, BoothSetDTO{Size: s.Size, Price: s.Price})
	}
	for _, it := range c.Items {
		out.Items = append(out.Items, BoothItemDTO{SKUID: it.SKUID, Name: it.Name, Price: it.Price})
	}
	return out
}

// BoothLotDTO is how much of a line came out of one lot.
type BoothLotDTO struct {
	LotID     uint   `json:"lotId"`
	LotNumber string `json:"lotNumber"`
	Qty       int    `json:"qty"`
}

// BoothSaleLineDTO is one line of a bill.
type BoothSaleLineDTO struct {
	SKUID     uint          `json:"skuId"`
	SKUCode   string        `json:"skuCode"`
	Name      string        `json:"name"`
	Kind      string        `json:"kind"`
	Qty       int           `json:"qty"`
	UnitPrice float64       `json:"unitPrice"`
	Amount    float64       `json:"amount"`
	Lots      []BoothLotDTO `json:"lots"`
}

// BoothTreatPartDTO is one set size used on a bill.
type BoothTreatPartDTO struct {
	Size      int     `json:"size"`
	Count     int     `json:"count"`
	UnitPrice float64 `json:"unitPrice"`
}

// BoothSaleDTO is a bill.
type BoothSaleDTO struct {
	ID            uint                `json:"id"`
	SaleNo        string              `json:"saleNo"`
	ClientRef     string              `json:"clientRef"`
	WarehouseID   uint                `json:"warehouseId"`
	SellerID      uint                `json:"sellerId"`
	SellerName    string              `json:"sellerName"`
	Lines         []BoothSaleLineDTO  `json:"lines"`
	Bags          int                 `json:"bags"`
	TreatLabel    string              `json:"treatLabel"`
	TreatParts    []BoothTreatPartDTO `json:"treatParts"`
	TreatTotal    float64             `json:"treatTotal"`
	ItemsTotal    float64             `json:"itemsTotal"`
	Subtotal      float64             `json:"subtotal"`
	Discount      float64             `json:"discount"`
	Total         float64             `json:"total"`
	PaymentMethod string              `json:"paymentMethod"`
	CashReceived  float64             `json:"cashReceived"`
	Change        float64             `json:"change"`
	Status        string              `json:"status"`
	VoidedAt      *time.Time          `json:"voidedAt"`
	VoidedBy      string              `json:"voidedBy"`
	VoidReason    string              `json:"voidReason"`
	CreatedAt     time.Time           `json:"createdAt"`
}

func boothLines(lines []domainBooth.SaleLine) []BoothSaleLineDTO {
	out := make([]BoothSaleLineDTO, 0, len(lines))
	for _, l := range lines {
		lots := make([]BoothLotDTO, 0, len(l.Lots))
		for _, a := range l.Lots {
			lots = append(lots, BoothLotDTO{LotID: a.LotID, LotNumber: a.LotNumber, Qty: a.Qty})
		}
		out = append(out, BoothSaleLineDTO{SKUID: l.SKUID, SKUCode: l.SKUCode, Name: l.Name, Kind: l.Kind,
			Qty: l.Qty, UnitPrice: l.UnitPrice, Amount: l.Amount, Lots: lots})
	}
	return out
}

func boothParts(parts []domainBooth.TreatPart) []BoothTreatPartDTO {
	out := make([]BoothTreatPartDTO, 0, len(parts))
	for _, p := range parts {
		out = append(out, BoothTreatPartDTO{Size: p.Size, Count: p.Count, UnitPrice: p.UnitPrice})
	}
	return out
}

// BoothSaleFromDomain converts a stored bill.
func BoothSaleFromDomain(s *domainBooth.Sale) BoothSaleDTO {
	return BoothSaleDTO{
		ID: s.ID, SaleNo: s.SaleNo, ClientRef: s.ClientRef, WarehouseID: s.WarehouseID,
		SellerID: s.SellerID, SellerName: s.SellerName, Lines: boothLines(s.Lines),
		Bags: s.Bags, TreatLabel: s.TreatLabel, TreatParts: boothParts(s.TreatParts),
		TreatTotal: s.TreatTotal, ItemsTotal: s.ItemsTotal, Subtotal: s.Subtotal,
		Discount: s.Discount, Total: s.Total, PaymentMethod: s.PaymentMethod,
		CashReceived: s.CashReceived, Change: s.Change, Status: s.Status,
		VoidedAt: s.VoidedAt, VoidedBy: s.VoidedBy, VoidReason: s.VoidReason, CreatedAt: s.CreatedAt,
	}
}

// BoothBillDTO is a priced cart (quote, or the current price on a conflict).
type BoothBillDTO struct {
	Bags       int                 `json:"bags"`
	Lines      []BoothSaleLineDTO  `json:"lines"`
	TreatParts []BoothTreatPartDTO `json:"treatParts"`
	TreatLabel string              `json:"treatLabel"`
	TreatTotal float64             `json:"treatTotal"`
	ItemsTotal float64             `json:"itemsTotal"`
	Subtotal   float64             `json:"subtotal"`
	Discount   float64             `json:"discount"`
	Total      float64             `json:"total"`
}

// BoothBillFromParts converts a priced cart.
func BoothBillFromParts(bags int, lines []domainBooth.SaleLine, parts []domainBooth.TreatPart, label string,
	treat, items, subtotal, discount, total float64) BoothBillDTO {
	return BoothBillDTO{Bags: bags, Lines: boothLines(lines), TreatParts: boothParts(parts), TreatLabel: label,
		TreatTotal: treat, ItemsTotal: items, Subtotal: subtotal, Discount: discount, Total: total}
}
