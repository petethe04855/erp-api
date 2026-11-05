package catalog

import (
	"context"

	domainSKU "chawy-erp-api/internal/domain/sku"
	appErrors "chawy-erp-api/pkg/errors"
)

// SKUAccessoryRecord — response view, JSON fields identical to the record the
// workspace handler used to build inline (erp-web-v2 contract).
type SKUAccessoryRecord struct {
	ID           uint   `json:"id"`
	SKU          string `json:"sku"`
	AccessorySKU string `json:"accessorySku"`
	Quantity     int    `json:"quantity"`
	Note         string `json:"note"`
	Name         string `json:"name,omitempty"`
}

// ProductRecord — response view, JSON fields identical to the record the
// workspace handler used to build inline.
type ProductRecord struct {
	ID              uint                 `json:"id"`
	SKU             string               `json:"sku"`
	Name            string               `json:"name"`
	Type            string               `json:"type"`
	Barcode         string               `json:"barcode"`
	BaseUnit        string               `json:"baseUnit"`
	RetailPrice     float64              `json:"retailPrice"`
	WholesalePrice  float64              `json:"wholesalePrice"`
	Cost            float64              `json:"cost"`
	Stock           int                  `json:"stock"`
	ReservedQty     int                  `json:"reservedQty"`
	UsedQty         int                  `json:"usedQty"`
	Reorder         int                  `json:"reorder"`
	IsBundle        bool                 `json:"isBundle"`
	IsActive        bool                 `json:"isActive"`
	Available       int                  `json:"available"`
	BundleAvailable *int                 `json:"bundleAvailable,omitempty"`
	Image           string               `json:"image"`
	Accessories     []SKUAccessoryRecord `json:"accessories,omitempty"`
	CreatedAt       string               `json:"createdAt"`
	LastReceivedAt  *string              `json:"lastReceivedAt"`
	ReceiptCount    int                  `json:"receiptCount"`
}

// ResolvedSKU is one row of the /skus/resolve batch response.
type ResolvedSKU struct {
	Sku   string `json:"sku"`
	Id    uint   `json:"id"`
	Name  string `json:"name"`
	Found bool   `json:"found"`
}

// ImportRowError / ImportSummary mirror the XLSX import response contract.
type ImportRowError struct {
	Row   int    `json:"row"`
	SKU   string `json:"sku"`
	Error string `json:"error"`
}

type ImportSummary struct {
	TotalRows int              `json:"totalRows"`
	Created   int              `json:"created"`
	Updated   int              `json:"updated"`
	Failed    int              `json:"failed"`
	Errors    []ImportRowError `json:"errors"`
}

// ComponentLine / AccessoryLine carry bundle component and accessory payloads
// (JSON tags mirror the request body so the handler can parse straight in).
type ComponentLine struct {
	ComponentSKU string `json:"componentSku"`
	SKU          string `json:"sku"`
	Quantity     int    `json:"quantity"`
	Qty          int    `json:"qty"`
	Note         string `json:"note"`
}

type AccessoryLine struct {
	AccessorySKU string `json:"accessorySku"`
	SKU          string `json:"sku"`
	Quantity     int    `json:"quantity"`
	Qty          int    `json:"qty"`
	Note         string `json:"note"`
}

// CreateInput mirrors the create-product request body.
type CreateInput struct {
	SKU             string          `json:"sku"`
	Name            string          `json:"name"`
	Barcode         string          `json:"barcode"`
	Type            string          `json:"type"`
	BaseUnit        string          `json:"baseUnit"`
	RetailPrice     float64         `json:"retailPrice"`
	Cost            float64         `json:"cost"`
	IsBundle        bool            `json:"isBundle"` // always forced false by spec
	Image           string          `json:"image"`
	InitialQuantity int             `json:"initialQuantity"`
	Stock           int             `json:"stock"`
	Components      []ComponentLine `json:"components"`
	Accessories     []AccessoryLine `json:"accessories"`
}

// UpdateInput mirrors the update-product request body; pointer fields mean
// "present in the payload" (FULL-11 partial-field rules).
type UpdateInput struct {
	SKU         *string         `json:"sku"`
	Name        *string         `json:"name"`
	Barcode     *string         `json:"barcode"`
	Type        *string         `json:"type"`
	RetailPrice *float64        `json:"retailPrice"`
	Cost        *float64        `json:"cost"`
	IsBundle    *bool           `json:"isBundle"`
	Image       *string         `json:"image"`
	Status      *string         `json:"status"`
	Components  []ComponentLine `json:"components"`
	Accessories []AccessoryLine `json:"accessories"`
}

// ImportLine is one parsed spreadsheet data row handed to Import; the usecase
// performs parsing/normalisation of values and all business validation.
type ImportLine struct {
	Row      int
	SKU      string
	Name     string
	Barcode  string
	Category string
	Price    string
	Cost     string
	Stock    string
	Reorder  string
}

// DBError carries the exact HTTP-facing response the old inline handler code
// produced for a database failure: a fixed status, an optional machine error
// code and the full composed message (including the raw error text).
type DBError struct {
	Status  int
	Code    string
	Message string
}

func (e *DBError) Error() string { return e.Message }

func dbErr500(message string) *DBError {
	return &DBError{Status: 500, Message: message}
}

func dbErr400(message, code string) *DBError {
	return &DBError{Status: 400, Message: message, Code: code}
}

func appErr(code, message string, statusCode int) *appErrors.AppError {
	return appErrors.NewAppError(code, message, statusCode)
}

// ReceiptStatsSource is the narrow slice of the SKU repository the list view
// needs for LastReceivedAt/ReceiptCount aggregation.
type ReceiptStatsSource interface {
	GetReceiptStatsBatch(ctx context.Context, skuIDs []uint) (map[uint]domainSKU.SKUBatchReceiptStat, error)
}
