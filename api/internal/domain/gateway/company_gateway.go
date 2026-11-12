package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// CompanyGateway is the display record of companies. Companies have no
// OpenFGA tuples: they decide a member's affiliation at add time, nothing else.
type CompanyGateway interface {
	// Create inserts a subcontractor company. A tenant's own company is
	// inserted by TenantGateway.Create.
	Create(ctx context.Context, c entity.Company) error
	Get(ctx context.Context, id string) (entity.Company, error)
	// GetOwn returns the tenant's own company.
	GetOwn(ctx context.Context, tenantID string) (entity.Company, error)
	List(ctx context.Context) ([]entity.Company, error)
	// ListForTenant returns the tenant's own company and the companies the
	// tenant created.
	ListForTenant(ctx context.Context, tenantID string) ([]entity.Company, error)
	// CountMembersIn counts memberships of the tenant held by users of the companies.
	CountMembersIn(ctx context.Context, tenantID string, companyIDs []string) (int, error)
	// Merge moves every user of fromIDs to intoID and deletes the from
	// companies, in one transaction.
	Merge(ctx context.Context, intoID string, fromIDs []string) error
}
