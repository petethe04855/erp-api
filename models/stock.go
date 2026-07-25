package models

// StockLot represents tracking at a lot level (for FEFO)
type StockLot struct {
	ID             uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	Code           string  `json:"code"`
	ProductID      uint    `gorm:"index" json:"productId"`
	SKU            string  `gorm:"index" json:"sku"`
	Lot            string  `json:"lot"`
	Qty            int     `json:"qty"`
	RemainingQty   int     `json:"remainingQty"`
	LandedUnitCost float64 `json:"landedUnitCost"`
	ExpiryDate     string  `json:"expiryDate"`
	ReceivedDate   string  `json:"receivedDate"`
	GoodsReceiveID *uint   `gorm:"index" json:"goodsReceiveId,omitempty"`
	GrRef          string  `json:"grRef"`
	PurchaseOrderID *uint  `gorm:"index" json:"purchaseOrderId,omitempty"`
	PoRef          string  `json:"poRef"`
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
}

// StockReturn represents returned client stock
type StockReturn struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code         string `json:"code"`
	SalesOrderID *uint  `gorm:"index" json:"salesOrderId,omitempty"`
	SoRef        string `json:"soRef"`
	ProductID    uint   `gorm:"index" json:"productId"`
	SKU          string `json:"sku"`
	SkuName      string `json:"skuName"`
	Qty          int    `json:"qty"`
	Condition    string `json:"condition"` // ดี, เสียหาย
	Reason       string `json:"reason"`
	Note         string `json:"note"`
	Date         string `json:"date"`
	ReturnedBy   string `json:"returnedBy"`
	Refunded     bool   `json:"refunded"`
	Channel      string `json:"channel"`
	Status       string `json:"status"` // Pending, Completed, Cancelled
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
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string `json:"code"`
	ProductID uint   `gorm:"index" json:"productId"`
	SKU       string `json:"sku"`
	Type      string `json:"type"` // IN, OUT
	Qty       int    `json:"qty"`
	RefDoc    string `json:"refDoc"`
	RefDocType string `json:"refDocType"`
	RefDocID  *uint  `json:"refDocId,omitempty"`
	Date      string `json:"date"`
	Note      string `json:"note"`
	ChangedBy string `json:"changedBy"`
}
