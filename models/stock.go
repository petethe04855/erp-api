package models

// StockLot represents tracking at a lot level (for FEFO)
type StockLot struct {
	ID              uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code            string  `json:"code"`
	ProductID       uint    `gorm:"index" json:"productId"`
	SKU             string  `gorm:"index" json:"sku"`
	Lot             string  `json:"lot"`
	Qty             int     `json:"qty"`
	RemainingQty    int     `json:"remainingQty"`
	LandedUnitCost  float64 `json:"landedUnitCost"`
	ExpiryDate      string  `json:"expiryDate"`
	SupplierLot     string  `json:"supplierLot"`
	QCStatus        string  `json:"qcStatus"`
	ReceivedDate    string  `json:"receivedDate"`
	GoodsReceiveID  *uint   `gorm:"index" json:"goodsReceiveId,omitempty"`
	GrRef           string  `json:"grRef"`
	PurchaseOrderID *uint   `gorm:"index" json:"purchaseOrderId,omitempty"`
	PoRef           string  `json:"poRef"`
}

// GoodsIssue represents stock issues not related to orders
type GoodsIssue struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string `json:"code"`
	ProductID uint   `gorm:"index" json:"productId"`
	SKU       string `json:"sku"`
	SkuName   string `json:"skuName"`
	Qty       int    `json:"qty"`
	Reason    string `json:"reason"`
	Note      string `json:"note"`
	Date      string `json:"date"`
	IssuedBy  string `json:"issuedBy"`
	Channel   string `json:"channel"`
	OrderRef  string `gorm:"index" json:"orderRef"`
}

// ProductionRun records the conversion of BOM materials into finished goods.
// The output is received as a normal stock lot; the inputs are deducted FEFO.
type ProductionRun struct {
	ID         uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code       string `gorm:"uniqueIndex" json:"code"`
	ProductID  uint   `gorm:"index" json:"productId"`
	SKU        string `gorm:"index" json:"sku"`
	SkuName    string `json:"skuName"`
	BOMCode    string `json:"bomCode"`
	Qty        int    `json:"qty"`
	Lot        string `json:"lot"`
	ExpiryDate string `json:"expiryDate"`
	Date       string `json:"date"`
	Note       string `json:"note"`
	ProducedBy string `json:"producedBy"`
}

// StockReturn represents returned client stock
type StockReturn struct {
	ID              uint                    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code            string                  `json:"code"`
	SalesOrderID    *uint                   `gorm:"index" json:"salesOrderId,omitempty"`
	SoRef           string                  `json:"soRef"`
	ProductID       uint                    `gorm:"index" json:"productId"`
	SKU             string                  `json:"sku"`
	SkuName         string                  `json:"skuName"`
	Qty             int                     `json:"qty"`
	Condition       string                  `json:"condition"` // ดี, เสียหาย
	Reason          string                  `json:"reason"`
	Note            string                  `json:"note"`
	Date            string                  `json:"date"`
	ReturnedBy      string                  `json:"returnedBy"`
	Refunded        bool                    `json:"refunded"`
	Channel         string                  `json:"channel"`
	Status          string                  `json:"status"` // Pending, Completed, Cancelled
	QuarantineQty   int                     `gorm:"default:0" json:"quarantineQty"`
	QCStatus        string                  `json:"qcStatus"` // Pending, Passed, Failed
	CreditAmount    float64                 `gorm:"default:0" json:"creditAmount"`
	CreditSubtotal  float64                 `gorm:"default:0" json:"creditSubtotal"`
	CreditVATAmount float64                 `gorm:"default:0" json:"creditVatAmount"`
	CreditDiscount  float64                 `gorm:"default:0" json:"creditDiscount"`
	TotalCost       float64                 `gorm:"default:0" json:"totalCost"`
	CreditNoteID    *uint                   `gorm:"index" json:"creditNoteId,omitempty"`
	CreditNoteRef   string                  `gorm:"index" json:"creditNoteRef"`
	Allocations     []ReturnStockAllocation `gorm:"foreignKey:StockReturnID" json:"allocations"`
}

// ReturnStockAllocation traces returned units back to the exact lot and cost used by the sale.
type ReturnStockAllocation struct {
	ID                     uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	StockReturnID          uint    `gorm:"uniqueIndex:idx_return_sales_allocation;not null" json:"stockReturnId"`
	SalesStockAllocationID uint    `gorm:"uniqueIndex:idx_return_sales_allocation;not null" json:"salesStockAllocationId"`
	StockLotID             uint    `gorm:"index;not null" json:"stockLotId"`
	SKU                    string  `gorm:"index" json:"sku"`
	Lot                    string  `json:"lot"`
	Qty                    int     `json:"qty"`
	UnitCost               float64 `json:"unitCost"`
	TotalCost              float64 `json:"totalCost"`
	Restocked              bool    `json:"restocked"`
}

// StockAdjustment represents physical count changes
type StockAdjustment struct {
	ID        uint                  `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string                `json:"code"`
	Date      string                `json:"date"`
	CheckedBy string                `json:"checkedBy"`
	Note      string                `json:"note"`
	Items     []StockAdjustmentItem `gorm:"foreignKey:StockAdjustmentID" json:"items"`
}

// StockAdjustmentItem contains details of adjusted stocks
type StockAdjustmentItem struct {
	ID                uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	StockAdjustmentID uint   `gorm:"index" json:"stockAdjustmentId"`
	ProductID         uint   `gorm:"index" json:"productId"`
	SKU               string `json:"sku"`
	SkuName           string `json:"skuName"`
	SystemQty         int    `json:"systemQty"`
	ActualQty         int    `json:"actualQty"`
	Variance          int    `json:"variance"`
}

// StockTransfer represents transfers between warehouse locations
type StockTransfer struct {
	ID            uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code          string `json:"code"`
	ProductID     uint   `gorm:"index" json:"productId"`
	SKU           string `json:"sku"`
	SkuName       string `json:"skuName"`
	Qty           int    `json:"qty"`
	FromLocation  string `json:"fromLocation"`
	ToLocation    string `json:"toLocation"`
	Note          string `json:"note"`
	Date          string `json:"date"`
	TransferredBy string `json:"transferredBy"`
}

// StockMovement represents tracking stock history
type StockMovement struct {
	ID         uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code       string `json:"code"`
	ProductID  uint   `gorm:"index" json:"productId"`
	SKU        string `json:"sku"`
	Type       string `json:"type"` // IN, OUT
	Qty        int    `json:"qty"`
	RefDoc     string `json:"refDoc"`
	RefDocType string `json:"refDocType"`
	RefDocID   *uint  `json:"refDocId,omitempty"`
	Date       string `json:"date"`
	Note       string `json:"note"`
	ChangedBy  string `json:"changedBy"`
}
