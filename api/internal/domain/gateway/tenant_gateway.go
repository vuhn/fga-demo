package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type TenantGateway interface {
	// Create inserts the tenant with every product disabled.
	Create(ctx context.Context, t model.Tenant) error
	Get(ctx context.Context, id string) (model.Tenant, error)
	List(ctx context.Context) ([]model.Tenant, error)
	ListByIDs(ctx context.Context, ids []string) ([]model.Tenant, error)
	UpdateName(ctx context.Context, id, name string) (model.Tenant, error)
}
