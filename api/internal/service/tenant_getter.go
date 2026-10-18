package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type tenantGetter struct {
	tenants gateway.TenantGateway
	authz   usecase.Authorizer
}

func NewTenantGetter(tenants gateway.TenantGateway, authz usecase.Authorizer) usecase.TenantGetter {
	return &tenantGetter{tenants: tenants, authz: authz}
}

// Get is the platform or any reader of the tenant; anyone else gets 404.
func (s *tenantGetter) Get(ctx context.Context, actor, id string) (entity.Tenant, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_read_tenant", id, entity.ErrNotFound); err != nil {
		return entity.Tenant{}, err
	}
	return s.tenants.Get(ctx, id)
}
