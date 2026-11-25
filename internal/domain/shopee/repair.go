package shopee

import "time"

// OrderItemRepair is one line whose stored quantity or variation differs from
// the Order export. Only these fields are ever rewritten: the SKU (which users
// may have corrected and confirmed) is left alone.
type OrderItemRepair struct {
	ItemID       uint   `json:"item_id"`
	SKU          string `json:"sku"`
	ProductName  string `json:"product_name"`
	OldQty       int    `json:"old_qty"`
	NewQty       int    `json:"new_qty"`
	OldVariation string `json:"old_variation"`
	NewVariation string `json:"new_variation"`
}

// OrderRepair is what re-reading an Order export would change on one stored order.
type OrderRepair struct {
	OrderID      string            `json:"order_id"`
	OldOrderDate time.Time         `json:"old_order_date"`
	NewOrderDate time.Time         `json:"new_order_date"`
	DateChanged  bool              `json:"date_changed"`
	Items        []OrderItemRepair `json:"items"`
}

// ShopeeOrderRepair records one field changed by a repair, so a repair can be
// audited and, if needed, reversed from the old values.
type ShopeeOrderRepair struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	BatchID    string    `json:"batch_id" gorm:"index;size:64;not null"`
	OrderID    string    `json:"order_id" gorm:"index;size:64;not null"`
	ItemID     *uint     `json:"item_id"`
	Field      string    `json:"field" gorm:"size:32;not null"` // order_date | qty | variation_name
	OldValue   string    `json:"old_value" gorm:"size:500"`
	NewValue   string    `json:"new_value" gorm:"size:500"`
	Actor      string    `json:"actor" gorm:"size:255"`
	SourceFile string    `json:"source_file" gorm:"size:255"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ShopeeOrderRepair) TableName() string { return "shopee_order_repairs" }
