package models

// PurchaseRequest represents a PR
type PurchaseRequest struct {
	ID         string                `gorm:"primaryKey" json:"id"`
	Requester  string                `json:"requester"`
	Reason     string                `json:"reason"`
	NeededDate string                `json:"neededDate"`
	Date       string                `json:"date"`
	Items      []PurchaseRequestItem `gorm:"foreignKey:RequestID" json:"items"`
	Status     string                `json:"status"`
	PoRef      string                `json:"poRef"`
}

// PurchaseRequestItem contains details of items inside a PR
type PurchaseRequestItem struct {
	ID        uint   `gorm:"primaryKey" json:"-"`
	RequestID string `gorm:"index" json:"-"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Qty       int    `json:"qty"`
	Note      string `json:"note"`
}

// PurchaseOrder represents a PO
type PurchaseOrder struct {
	ID         string              `gorm:"primaryKey" json:"id"`
	Supplier   string              `json:"supplier"`
	EtaDate    string              `json:"etaDate"`
	Date       string              `json:"date"`
	Items      []PurchaseOrderItem `gorm:"foreignKey:OrderID" json:"items"`
	Status     string              `json:"status"`
	PrRef      string              `json:"prRef"`
	TotalCost  float64             `json:"totalCost"`
	AuditTrail []AuditEvent        `gorm:"polymorphic:Owner;" json:"auditTrail"`
}

// PurchaseOrderItem represents items inside a PO
type PurchaseOrderItem struct {
	ID          uint    `gorm:"primaryKey" json:"-"`
	OrderID     string  `gorm:"index" json:"-"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	Qty         int     `json:"qty"`
	UnitCost    float64 `json:"unitCost"`
	ReceivedQty int     `json:"receivedQty"`
}

// LandedCostLine = ค่าใช้จ่ายแฝงต่อ 1 ใบรับเข้า ที่ต้องปันเข้าต้นทุนวัตถุดิบ
type LandedCostLine struct {
	ID          uint    `gorm:"primaryKey" json:"-"`
	ReceiveID   string  `gorm:"index" json:"-"`
	Type        string  `json:"type"`        // freight, duty, shipping, other
	Amount      float64 `json:"amount"`      // บาท
	Allocatable bool    `gorm:"default:true" json:"allocatable"` // true = ปันเข้าต้นทุน, false = บันทึกเป็นค่าใช้จ่ายเฉยๆ
	Note        string  `json:"note"`
}

// GoodsReceive represents a GR
type GoodsReceive struct {
	ID          string             `gorm:"primaryKey" json:"id"`
	PoRef       string             `json:"poRef"`
	ReceiveDate string             `json:"receiveDate"`
	Items       []GoodsReceiveItem `gorm:"foreignKey:ReceiveID" json:"items"`
	LandedCosts []LandedCostLine   `gorm:"foreignKey:ReceiveID" json:"landedCosts"`
	AuditTrail  []AuditEvent       `gorm:"polymorphic:Owner;" json:"auditTrail"`
}

// GoodsReceiveItem represents items inside a GR
type GoodsReceiveItem struct {
	ID             uint    `gorm:"primaryKey" json:"-"`
	ReceiveID      string  `gorm:"index" json:"-"`
	SKU            string  `json:"sku"`
	QtyReceived    int     `json:"qtyReceived"`
	Lot            string  `json:"lot"`
	ExpiryDate     string  `json:"expiryDate"`
	LandedUnitCost float64 `json:"landedUnitCost"` // ราคาซื้อ + ค่าขนส่งปันส่วน ต่อหน่วย (คำนวณตอน GR)
}
