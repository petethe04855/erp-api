package models

// PurchaseRequest represents a PR
type PurchaseRequest struct {
	ID         uint                  `gorm:"primaryKey;autoIncrement" json:"id"`
	Code       string                `gorm:"uniqueIndex;not null" json:"code"`
	Requester  string                `json:"requester"`
	Reason     string                `json:"reason"`
	NeededDate string                `json:"neededDate"`
	Date       string                `json:"date"`
	Items      []PurchaseRequestItem `gorm:"foreignKey:PurchaseRequestID" json:"items"`
	Status     string                `json:"status"`
	PoRef      string                `json:"poRef"`
}

// PurchaseRequestItem contains details of items inside a PR
type PurchaseRequestItem struct {
	ID                uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	PurchaseRequestID uint   `gorm:"index" json:"purchaseRequestId"`
	ProductID         uint   `gorm:"index" json:"productId"`
	SKU               string `json:"sku"`
	Name              string `json:"name"`
	Qty               int    `json:"qty"`
	Note              string `json:"note"`
}

// PurchaseOrder represents a PO
type PurchaseOrder struct {
	ID                uint                `gorm:"primaryKey;autoIncrement" json:"id"`
	Code              string              `gorm:"uniqueIndex;not null" json:"code"`
	Supplier          string              `json:"supplier"`
	EtaDate           string              `json:"etaDate"`
	Date              string              `json:"date"`
	Items             []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID" json:"items"`
	Status            string              `json:"status"`
	PurchaseRequestID *uint               `gorm:"index" json:"purchaseRequestId,omitempty"`
	PrRef             string              `json:"prRef"`
	TotalCost         float64             `json:"totalCost"`
	AuditTrail        []AuditEvent        `gorm:"polymorphic:Owner;" json:"auditTrail"`
}

// PurchaseOrderItem represents items inside a PO
type PurchaseOrderItem struct {
	ID              uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	PurchaseOrderID uint    `gorm:"index" json:"purchaseOrderId"`
	ProductID       uint    `gorm:"index" json:"productId"`
	SKU             string  `json:"sku"`
	Name            string  `json:"name"`
	Qty             int     `json:"qty"`
	UnitCost        float64 `json:"unitCost"`
	ReceivedQty     int     `json:"receivedQty"`
}

// LandedCostLine = ค่าใช้จ่ายแฝงต่อ 1 ใบรับเข้า ที่ต้องปันเข้าต้นทุนวัตถุดิบ
type LandedCostLine struct {
	ID             uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	GoodsReceiveID uint    `gorm:"index" json:"goodsReceiveId"`
	Type           string  `json:"type"`                            // freight, duty, shipping, other
	Amount         float64 `json:"amount"`                          // บาท
	Allocatable    bool    `gorm:"default:true" json:"allocatable"` // true = ปันเข้าต้นทุน, false = บันทึกเป็นค่าใช้จ่ายเฉยๆ
	Note           string  `json:"note"`
}

// GoodsReceive represents a GR
type GoodsReceive struct {
	ID              uint               `gorm:"primaryKey;autoIncrement" json:"id"`
	Code            string             `gorm:"uniqueIndex;not null" json:"code"`
	PurchaseOrderID *uint              `gorm:"index" json:"purchaseOrderId,omitempty"`
	PoRef           string             `json:"poRef"`
	ReceiveDate     string             `json:"receiveDate"`
	Note            string             `json:"note"`
	Items           []GoodsReceiveItem `gorm:"foreignKey:GoodsReceiveID" json:"items"`
	LandedCosts     []LandedCostLine   `gorm:"foreignKey:GoodsReceiveID" json:"landedCosts"`
	AuditTrail      []AuditEvent       `gorm:"polymorphic:Owner;" json:"auditTrail"`
}

// GoodsReceiveItem represents items inside a GR
type GoodsReceiveItem struct {
	ID             uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	GoodsReceiveID uint    `gorm:"index" json:"goodsReceiveId"`
	ProductID      uint    `gorm:"index" json:"productId"`
	SKU            string  `json:"sku"`
	QtyReceived    int     `json:"qtyReceived"`
	Lot            string  `json:"lot"`
	ExpiryDate     string  `json:"expiryDate"`
	SupplierLot    string  `json:"supplierLot"`
	QCStatus       string  `json:"qcStatus"`
	AcceptedQty    int     `json:"acceptedQty"`
	RejectedQty    int     `json:"rejectedQty"`
	QCNote         string  `json:"qcNote"`
	LandedUnitCost float64 `json:"landedUnitCost"` // ยังไม่คำนวณในขั้นตอน Stock Receipt
}
