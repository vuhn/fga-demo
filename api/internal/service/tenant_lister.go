package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type tenantLister struct {
	tenants gateway.TenantGateway
	authz   usecase.Authorizer
}

func NewTenantLister(tenants gateway.TenantGateway, authz usecase.Authorizer) usecase.TenantLister {
	return &tenantLister{tenants: tenants, authz: authz}
}

// List is the platform's tenant directory.
func (s *tenantLister) List(ctx context.Context, actor string) ([]entity.Tenant, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return nil, err
	}
	return s.tenants.List(ctx)
}
