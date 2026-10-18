package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type TenantGateway interface {
	// Create inserts the tenant with every product disabled.
	Create(ctx context.Context, t entity.Tenant) error
	Get(ctx context.Context, id string) (entity.Tenant, error)
	List(ctx context.Context) ([]entity.Tenant, error)
	ListByIDs(ctx context.Context, ids []string) ([]entity.Tenant, error)
	UpdateName(ctx context.Context, id, name string) (entity.Tenant, error)
}
