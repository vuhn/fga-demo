package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type tenantUpdater struct {
	tenants gateway.TenantGateway
	authz   usecase.Authorizer
}

func NewTenantUpdater(tenants gateway.TenantGateway, authz usecase.Authorizer) usecase.TenantUpdater {
	return &tenantUpdater{tenants: tenants, authz: authz}
}

// UpdateName is platform only.
func (s *tenantUpdater) UpdateName(ctx context.Context, actor, id, name string) (model.Tenant, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return model.Tenant{}, err
	}
	name, err := model.CleanName("name", name)
	if err != nil {
		return model.Tenant{}, err
	}
	return s.tenants.UpdateName(ctx, id, name)
}
