package models

// SamplingCampaign represents free sampling drives
type SamplingCampaign struct {
	ID         uint                `gorm:"primaryKey;autoIncrement" json:"id"`
	Code       string              `json:"code"`
	Name       string              `json:"name"`
	ProductID  *uint               `gorm:"index" json:"productId,omitempty"`
	SKU        string              `json:"sku"`
	SkuName    string              `json:"skuName"`
	TargetQty  int                 `json:"targetQty"`
	GivenQty   int                 `json:"givenQty"`
	Note       string              `json:"note"`
	StartDate  string              `json:"startDate"`
	EndDate    string              `json:"endDate"`
	Status     string              `json:"status"` // Active, Completed, Cancelled
	Recipients []SamplingRecipient `gorm:"foreignKey:SamplingCampaignID" json:"recipients"`
}

// SamplingRecipient represents users registered to get trial packs
type SamplingRecipient struct {
	ID                 uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	SamplingCampaignID uint   `gorm:"index" json:"samplingCampaignId"`
	Name               string `json:"name"`
	Contact            string `json:"contact"`
	QtyGiven           int    `json:"qtyGiven"`
	Date               string `json:"date"`
	Feedback           string `json:"feedback"`
	Converted          bool   `json:"converted"`
}
