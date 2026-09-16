package postgres

import (
	"context"

	domainSettings "chawy-erp-api/internal/domain/settings"

	"gorm.io/gorm"
)

type settingsRepository struct {
	db *gorm.DB
}

func NewSettingsRepository(db *gorm.DB) domainSettings.Repository {
	return &settingsRepository{db: db}
}

func defaultCompanySettings() domainSettings.CompanySettings {
	return domainSettings.CompanySettings{
		Name:          "Chawy Pet Food",
		TaxID:         "0123456789012",
		Address:       "123 ถ.สุขุมวิท แขวงคลองเตย เขตคลองเตย กรุงเทพฯ 10110",
		Phone:         "02-123-4567",
		Email:         "hello@chawypet.com",
		Website:       "www.chawypet.com",
		Currency:      "THB",
		VatRate:       7,
		InvoicePrefix: "INV-2026-",
		SoPrefix:      "SO-2026-",
		LogoURL:       "",
	}
}

func defaultNotificationSettings() domainSettings.NotificationSettings {
	return domainSettings.NotificationSettings{
		NearExpiry:     true,
		NearExpiryDays: 30,
		LowStock:       true,
		LatePO:         true,
		NewSO:          true,
		PaymentDue:     true,
	}
}

func defaultModuleSettings() domainSettings.ModuleSettings {
	return domainSettings.ModuleSettings{
		Quotation:        true,
		SalesOrders:      true,
		Invoice:          true,
		Returns:          true,
		PurchaseReq:      true,
		PurchaseOrder:    true,
		SkuMaster:        true,
		StockBalance:     true,
		GoodsReceive:     true,
		GoodsIssue:       true,
		StockTransfer:    true,
		StockCheck:       true,
		Expenses:         true,
		PlReport:         true,
		Budget:           true,
		TiktokOrders:     true,
		LiveContent:      true,
		ManualOrder:      true,
		TiktokCalculator: true,
		Sampling:         true,
		UserManagement:   true,
		TiktokSetup:      true,
	}
}

func defaultLivePayrollSettings() domainSettings.LivePayrollSettings {
	return domainSettings.LivePayrollSettings{
		HourlyRate: 120,
		ClipBonus:  100,
		StaffRates: map[string]int{},
	}
}

func (r *settingsRepository) GetSettings(ctx context.Context) (*domainSettings.Settings, error) {
	var company domainSettings.CompanySettings
	var notifications domainSettings.NotificationSettings
	var modules domainSettings.ModuleSettings
	var livePayroll domainSettings.LivePayrollSettings

	// Ensure default company settings exists
	if err := r.db.WithContext(ctx).First(&company).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			company = defaultCompanySettings()
			if err := r.db.WithContext(ctx).Create(&company).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Ensure default notification settings exists
	if err := r.db.WithContext(ctx).First(&notifications).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			notifications = defaultNotificationSettings()
			if err := r.db.WithContext(ctx).Create(&notifications).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Ensure default module settings exists
	if err := r.db.WithContext(ctx).First(&modules).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			modules = defaultModuleSettings()
			if err := r.db.WithContext(ctx).Create(&modules).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Ensure default live payroll settings exists
	if err := r.db.WithContext(ctx).First(&livePayroll).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			livePayroll = defaultLivePayrollSettings()
			if err := r.db.WithContext(ctx).Create(&livePayroll).Error; err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return &domainSettings.Settings{
		Company:       company,
		Notifications: notifications,
		Modules:       modules,
		LivePayroll:   livePayroll,
	}, nil
}

func (r *settingsRepository) UpdateCompany(ctx context.Context, comp *domainSettings.CompanySettings) (*domainSettings.CompanySettings, error) {
	var existing domainSettings.CompanySettings
	if err := r.db.WithContext(ctx).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			comp.ID = 0
			if err := r.db.WithContext(ctx).Create(comp).Error; err != nil {
				return nil, err
			}
			return comp, nil
		}
		return nil, err
	}

	existing.Name = comp.Name
	existing.TaxID = comp.TaxID
	existing.Address = comp.Address
	existing.Phone = comp.Phone
	existing.Email = comp.Email
	existing.Website = comp.Website
	existing.Currency = comp.Currency
	existing.VatRate = comp.VatRate
	existing.InvoicePrefix = comp.InvoicePrefix
	existing.SoPrefix = comp.SoPrefix
	existing.LogoURL = comp.LogoURL

	if err := r.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (r *settingsRepository) UpdateSettings(ctx context.Context, s *domainSettings.Settings) (*domainSettings.Settings, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Company
		var comp domainSettings.CompanySettings
		if err := tx.First(&comp).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				comp = s.Company
				if err := tx.Create(&comp).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			comp.Name = s.Company.Name
			comp.TaxID = s.Company.TaxID
			comp.Address = s.Company.Address
			comp.Phone = s.Company.Phone
			comp.Email = s.Company.Email
			comp.Website = s.Company.Website
			comp.Currency = s.Company.Currency
			comp.VatRate = s.Company.VatRate
			comp.InvoicePrefix = s.Company.InvoicePrefix
			comp.SoPrefix = s.Company.SoPrefix
			comp.LogoURL = s.Company.LogoURL
			if err := tx.Save(&comp).Error; err != nil {
				return err
			}
		}
		s.Company = comp

		// 2. Notifications
		var notif domainSettings.NotificationSettings
		if err := tx.First(&notif).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				notif = s.Notifications
				if err := tx.Create(&notif).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			notif.NearExpiry = s.Notifications.NearExpiry
			notif.NearExpiryDays = s.Notifications.NearExpiryDays
			notif.LowStock = s.Notifications.LowStock
			notif.LatePO = s.Notifications.LatePO
			notif.NewSO = s.Notifications.NewSO
			notif.PaymentDue = s.Notifications.PaymentDue
			if err := tx.Save(&notif).Error; err != nil {
				return err
			}
		}
		s.Notifications = notif

		// 3. Modules
		var mod domainSettings.ModuleSettings
		if err := tx.First(&mod).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				mod = s.Modules
				if err := tx.Create(&mod).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			mod = s.Modules
			mod.ID = 1
			if err := tx.Save(&mod).Error; err != nil {
				return err
			}
		}
		s.Modules = mod

		// 4. Live Payroll
		var payroll domainSettings.LivePayrollSettings
		if err := tx.First(&payroll).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				payroll = s.LivePayroll
				if err := tx.Create(&payroll).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			payroll.HourlyRate = s.LivePayroll.HourlyRate
			payroll.ClipBonus = s.LivePayroll.ClipBonus
			payroll.StaffRates = s.LivePayroll.StaffRates
			if err := tx.Save(&payroll).Error; err != nil {
				return err
			}
		}
		s.LivePayroll = payroll

		return nil
	})

	if err != nil {
		return nil, err
	}
	return s, nil
}
