package dto

import (
	domainSettings "chawy-erp-api/internal/domain/settings"
)

type UpdateSettingsRequest struct {
	Company       *domainSettings.CompanySettings      `json:"company"`
	Notifications *domainSettings.NotificationSettings `json:"notifications"`
	Modules       *domainSettings.ModuleSettings       `json:"modules"`
	LivePayroll   *domainSettings.LivePayrollSettings  `json:"livePayroll"`
}

type CompanySettingsResponse struct {
	Name                string  `json:"name"`
	TaxID               string  `json:"taxId"`
	Address             string  `json:"address"`
	Phone               string  `json:"phone"`
	Email               string  `json:"email"`
	Website             string  `json:"website"`
	Currency            string  `json:"currency"`
	VatRate             float64 `json:"vatRate"`
	InvoicePrefix       string  `json:"invoicePrefix"`
	SoPrefix            string  `json:"soPrefix"`
	LogoURL             string  `json:"logoUrl"`
	DefaultReorderPoint int     `json:"defaultReorderPoint"`
}

type NotificationSettingsResponse struct {
	NearExpiry     bool `json:"nearExpiry"`
	NearExpiryDays int  `json:"nearExpiryDays"`
	LowStock       bool `json:"lowStock"`
	LatePO         bool `json:"latePO"`
	NewSO          bool `json:"newSO"`
	PaymentDue     bool `json:"paymentDue"`
}

type ModuleSettingsResponse struct {
	Quotation        bool `json:"quotation"`
	SalesOrders      bool `json:"salesOrders"`
	Invoice          bool `json:"invoice"`
	Returns          bool `json:"returns"`
	PurchaseReq      bool `json:"purchaseReq"`
	PurchaseOrder    bool `json:"purchaseOrder"`
	SkuMaster        bool `json:"skuMaster"`
	StockBalance     bool `json:"stockBalance"`
	GoodsReceive     bool `json:"goodsReceive"`
	GoodsIssue       bool `json:"goodsIssue"`
	StockTransfer    bool `json:"stockTransfer"`
	StockCheck       bool `json:"stockCheck"`
	Expenses         bool `json:"expenses"`
	PlReport         bool `json:"plReport"`
	Budget           bool `json:"budget"`
	TiktokOrders     bool `json:"tiktokOrders"`
	LiveContent      bool `json:"liveContent"`
	ManualOrder      bool `json:"manualOrder"`
	TiktokCalculator bool `json:"tiktokCalculator"`
	Sampling         bool `json:"sampling"`
	UserManagement   bool `json:"userManagement"`
	TiktokSetup      bool `json:"tiktokSetup"`
}

type LivePayrollSettingsResponse struct {
	HourlyRate int            `json:"hourlyRate"`
	ClipBonus  int            `json:"clipBonus"`
	StaffRates map[string]int `json:"staffRates"`
}

type SettingsResponse struct {
	Company       CompanySettingsResponse      `json:"company"`
	Notifications NotificationSettingsResponse `json:"notifications"`
	Modules       ModuleSettingsResponse       `json:"modules"`
	LivePayroll   LivePayrollSettingsResponse  `json:"livePayroll"`
}

func NewSettingsResponse(entity *domainSettings.Settings) *SettingsResponse {
	if entity == nil {
		return nil
	}
	return &SettingsResponse{
		Company: CompanySettingsResponse{
			Name:                entity.Company.Name,
			TaxID:               entity.Company.TaxID,
			Address:             entity.Company.Address,
			Phone:               entity.Company.Phone,
			Email:               entity.Company.Email,
			Website:             entity.Company.Website,
			Currency:            entity.Company.Currency,
			VatRate:             entity.Company.VatRate,
			InvoicePrefix:       entity.Company.InvoicePrefix,
			SoPrefix:            entity.Company.SoPrefix,
			LogoURL:             entity.Company.LogoURL,
			DefaultReorderPoint: entity.Company.DefaultReorderPoint,
		},
		Notifications: NotificationSettingsResponse{
			NearExpiry:     entity.Notifications.NearExpiry,
			NearExpiryDays: entity.Notifications.NearExpiryDays,
			LowStock:       entity.Notifications.LowStock,
			LatePO:         entity.Notifications.LatePO,
			NewSO:          entity.Notifications.NewSO,
			PaymentDue:     entity.Notifications.PaymentDue,
		},
		Modules: ModuleSettingsResponse{
			Quotation:        entity.Modules.Quotation,
			SalesOrders:      entity.Modules.SalesOrders,
			Invoice:          entity.Modules.Invoice,
			Returns:          entity.Modules.Returns,
			PurchaseReq:      entity.Modules.PurchaseReq,
			PurchaseOrder:    entity.Modules.PurchaseOrder,
			SkuMaster:        entity.Modules.SkuMaster,
			StockBalance:     entity.Modules.StockBalance,
			GoodsReceive:     entity.Modules.GoodsReceive,
			GoodsIssue:       entity.Modules.GoodsIssue,
			StockTransfer:    entity.Modules.StockTransfer,
			StockCheck:       entity.Modules.StockCheck,
			Expenses:         entity.Modules.Expenses,
			PlReport:         entity.Modules.PlReport,
			Budget:           entity.Modules.Budget,
			TiktokOrders:     entity.Modules.TiktokOrders,
			LiveContent:      entity.Modules.LiveContent,
			ManualOrder:      entity.Modules.ManualOrder,
			TiktokCalculator: entity.Modules.TiktokCalculator,
			Sampling:         entity.Modules.Sampling,
			UserManagement:   entity.Modules.UserManagement,
			TiktokSetup:      entity.Modules.TiktokSetup,
		},
		LivePayroll: LivePayrollSettingsResponse{
			HourlyRate: entity.LivePayroll.HourlyRate,
			ClipBonus:  entity.LivePayroll.ClipBonus,
			StaffRates: entity.LivePayroll.StaffRates,
		},
	}
}
