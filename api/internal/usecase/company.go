package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// CompanyLister lists the companies a tenant can place members in: its own
// company and the subcontractor companies it created.
type CompanyLister interface {
	List(ctx context.Context, actor, tenantID string) ([]model.Company, error)
}

// AllCompanyLister lists every company. Platform only.
type AllCompanyLister interface {
	ListAll(ctx context.Context, actor string) ([]model.Company, error)
}

type CreateCompanyInput struct {
	Name  string
	TaxID string
}

// CompanyCreator creates an unclaimed subcontractor company on behalf of a tenant.
type CompanyCreator interface {
	Create(ctx context.Context, actor, tenantID string, in CreateCompanyInput) (model.Company, error)
}

// CompanyClaimer merges unclaimed companies into the claimer tenant's own
// company after the platform has verified ownership. Platform only.
type CompanyClaimer interface {
	Claim(ctx context.Context, actor, tenantID string, companyIDs []string) (model.Company, error)
}
