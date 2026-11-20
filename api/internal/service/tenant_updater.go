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

type tenantUpdater struct {
	tenants gateway.TenantGateway
	authz   helper.Authorizer
}

func NewTenantUpdater(tenants gateway.TenantGateway, authz helper.Authorizer) usecase.TenantUpdater {
	return &tenantUpdater{tenants: tenants, authz: authz}
}

// UpdateName is platform only.
func (s *tenantUpdater) UpdateName(ctx context.Context, in input.TenantUpdater) (output.TenantUpdater, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.TenantUpdater{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.TenantUpdater{}, err
	}
	got, err := s.tenants.UpdateName(ctx, in.ID, name)
	return output.TenantUpdater{Tenant: tenantOutput(got)}, err
}
