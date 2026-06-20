package models

// SamplingCampaign represents free sampling drives
type SamplingCampaign struct {
	ID         string              `gorm:"primaryKey" json:"id"`
	Name       string              `json:"name"`
	SKU        string              `json:"sku"`
	SkuName    string              `json:"skuName"`
	TargetQty  int                 `json:"targetQty"`
	GivenQty   int                 `json:"givenQty"`
	Note       string              `json:"note"`
	StartDate  string              `json:"startDate"`
	EndDate    string              `json:"endDate"`
	Status     string              `json:"status"` // Active, Completed, Cancelled
	Recipients []SamplingRecipient `gorm:"foreignKey:CampaignID" json:"recipients"`
}

// SamplingRecipient represents users registered to get trial packs
type SamplingRecipient struct {
	ID         string `gorm:"primaryKey" json:"id"`
	CampaignID string `gorm:"index" json:"-"`
	Name       string `json:"name"`
	Contact    string `json:"contact"`
	QtyGiven   int    `json:"qtyGiven"`
	Date       string `json:"date"`
	Feedback   string `json:"feedback"`
	Converted  bool   `json:"converted"`
}
