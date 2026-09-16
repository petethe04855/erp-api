package settings

import (
	"context"
	"strings"

	domainSettings "chawy-erp-api/internal/domain/settings"
	"chawy-erp-api/pkg/errors"
)

type UpdateSettingsInput struct {
	Company       *domainSettings.CompanySettings
	Notifications *domainSettings.NotificationSettings
	Modules       *domainSettings.ModuleSettings
	LivePayroll   *domainSettings.LivePayrollSettings
}

type Usecase interface {
	GetSettings(ctx context.Context) (*domainSettings.Settings, error)
	UpdateSettings(ctx context.Context, in UpdateSettingsInput) (*domainSettings.Settings, error)
}

type settingsUsecase struct {
	repo domainSettings.Repository
}

func NewSettingsUsecase(repo domainSettings.Repository) Usecase {
	return &settingsUsecase{repo: repo}
}

func (u *settingsUsecase) GetSettings(ctx context.Context) (*domainSettings.Settings, error) {
	return u.repo.GetSettings(ctx)
}

func (u *settingsUsecase) UpdateSettings(ctx context.Context, in UpdateSettingsInput) (*domainSettings.Settings, error) {
	current, err := u.repo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	if in.Company != nil {
		if strings.TrimSpace(in.Company.Name) == "" {
			return nil, errors.NewAppError("VALIDATION_ERROR", "Company name is required", 400)
		}
		current.Company = *in.Company
	}

	if in.Notifications != nil {
		current.Notifications = *in.Notifications
	}

	if in.Modules != nil {
		current.Modules = *in.Modules
	}

	if in.LivePayroll != nil {
		current.LivePayroll = *in.LivePayroll
	}

	return u.repo.UpdateSettings(ctx, current)
}
