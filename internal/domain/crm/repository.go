package crm

import (
	"context"
)

// Repository defines data access contract for CRM module
type Repository interface {
	GetTiktokProvinceReport(ctx context.Context, query ProvinceQuery) (*TiktokProvinceReport, error)
}
