package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type tenantGetter struct {
	tenants gateway.TenantGateway
	authz   helper.Authorizer
}

func NewTenantGetter(tenants gateway.TenantGateway, authz helper.Authorizer) usecase.TenantGetter {
	return &tenantGetter{tenants: tenants, authz: authz}
}

// Get is the platform or any reader of the tenant; anyone else gets 404.
func (s *tenantGetter) Get(ctx context.Context, in input.TenantGetter) (output.TenantGetter, error) {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_read_tenant", in.ID, model.ErrNotFound); err != nil {
		return output.TenantGetter{}, err
	}
	got, err := s.tenants.Get(ctx, in.ID)
	return output.TenantGetter{Tenant: tenantOutput(got)}, err
}
