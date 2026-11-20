package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type tenantLister struct {
	tenants gateway.TenantGateway
	authz   helper.Authorizer
}

func NewTenantLister(tenants gateway.TenantGateway, authz helper.Authorizer) usecase.TenantLister {
	return &tenantLister{tenants: tenants, authz: authz}
}

// List is the platform's tenant directory.
func (s *tenantLister) List(ctx context.Context, in input.TenantLister) (output.TenantLister, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.TenantLister{}, err
	}
	got, err := s.tenants.List(ctx)
	return output.TenantLister{Items: mapOutput(got, tenantOutput)}, err
}
