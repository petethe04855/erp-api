package models

// StockLot represents tracking at a lot level (for FEFO)
type StockLot struct {
	ID           string `gorm:"primaryKey" json:"id"`
	SKU          string `gorm:"index" json:"sku"`
	Lot          string `json:"lot"`
	Qty          int    `json:"qty"`
	RemainingQty int    `json:"remainingQty"`
	ExpiryDate   string `json:"expiryDate"`
	ReceivedDate string `json:"receivedDate"`
	GrRef        string `json:"grRef"`
	PoRef        string `json:"poRef"`
}

// GoodsIssue represents stock issues not related to orders
type GoodsIssue struct {
	ID       string `gorm:"primaryKey" json:"id"`
	SKU      string `json:"sku"`
	SkuName  string `json:"skuName"`
	Qty      int    `json:"qty"`
	Reason   string `json:"reason"`
	Note     string `json:"note"`
	Date     string `json:"date"`
	IssuedBy string `json:"issuedBy"`
}

// StockReturn represents returned client stock
type StockReturn struct {
	ID         string `gorm:"primaryKey" json:"id"`
	SoRef      string `json:"soRef"`
	SKU        string `json:"sku"`
	SkuName    string `json:"skuName"`
	Qty        int    `json:"qty"`
	Condition  string `json:"condition"` // ดี, เสียหาย
	Reason     string `json:"reason"`
	Note       string `json:"note"`
	Date       string `json:"date"`
	ReturnedBy string `json:"returnedBy"`
	Refunded   bool   `json:"refunded"`
}

// StockAdjustment represents physical count changes
type StockAdjustment struct {
	ID        string                 `gorm:"primaryKey" json:"id"`
	Date      string                 `json:"date"`
	CheckedBy string                 `json:"checkedBy"`
	Note      string                 `json:"note"`
	Items     []StockAdjustmentItem  `gorm:"foreignKey:AdjustmentID" json:"items"`
}

// StockAdjustmentItem contains details of adjusted stocks
type StockAdjustmentItem struct {
	ID           uint   `gorm:"primaryKey" json:"-"`
	AdjustmentID string `gorm:"index" json:"-"`
	SKU          string `json:"sku"`
	SkuName      string `json:"skuName"`
	SystemQty    int    `json:"systemQty"`
	ActualQty    int    `json:"actualQty"`
	Variance     int    `json:"variance"`
}

// StockTransfer represents transfers between warehouse locations
type StockTransfer struct {
	ID            string `gorm:"primaryKey" json:"id"`
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
	ID        string `gorm:"primaryKey" json:"id"`
	SKU       string `json:"sku"`
	Type      string `json:"type"` // IN, OUT
	Qty       int    `json:"qty"`
	RefDoc    string `json:"refDoc"`
	Date      string `json:"date"`
	Note      string `json:"note"`
	ChangedBy string `json:"changedBy"`
}
