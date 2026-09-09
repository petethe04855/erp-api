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
