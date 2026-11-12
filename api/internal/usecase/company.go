package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// CompanyLister lists the companies a tenant can place members in: its own
// company and the subcontractor companies it created.
type CompanyLister interface {
	List(ctx context.Context, actor, tenantID string) ([]entity.Company, error)
}

// AllCompanyLister lists every company. Platform only.
type AllCompanyLister interface {
	ListAll(ctx context.Context, actor string) ([]entity.Company, error)
}

type CreateCompanyInput struct {
	Name  string
	TaxID string
}

// CompanyCreator creates an unclaimed subcontractor company on behalf of a tenant.
type CompanyCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreateCompanyInput) (entity.Company, error)
}

// CompanyClaimer merges unclaimed companies into the claimer tenant's own
// company after the platform has verified ownership. Platform only.
type CompanyClaimer interface {
	Claim(ctx context.Context, actor, tenantID string, companyIDs []string) (entity.Company, error)
}
