package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

// CompanyLister lists the companies a tenant can place members in: its own
// company and the subcontractor companies it created.
type CompanyLister interface {
	List(ctx context.Context, in input.CompanyLister) (output.CompanyLister, error)
}

// AllCompanyLister lists every company. Platform only.
type AllCompanyLister interface {
	ListAll(ctx context.Context, in input.AllCompanyLister) (output.AllCompanyLister, error)
}

// CompanyCreator creates an unclaimed subcontractor company on behalf of a tenant.
type CompanyCreator interface {
	Create(ctx context.Context, in input.CompanyCreator) (output.CompanyCreator, error)
}

// CompanyClaimer merges unclaimed companies into the claimer tenant's own
// company after the platform has verified ownership. Platform only.
type CompanyClaimer interface {
	Claim(ctx context.Context, in input.CompanyClaimer) (output.CompanyClaimer, error)
}
