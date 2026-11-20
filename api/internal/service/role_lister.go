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

type roleLister struct {
	tenants gateway.TenantGateway
	roles   gateway.RoleGateway
	authz   helper.Authorizer
}

func NewRoleLister(tenants gateway.TenantGateway, roles gateway.RoleGateway, authz helper.Authorizer) usecase.RoleLister {
	return &roleLister{tenants: tenants, roles: roles, authz: authz}
}

func (s *roleLister) List(ctx context.Context, in input.RoleLister) (output.RoleLister, error) {
	// Member managers need the list to offer custom roles next to the built-in ones.
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_members", in.TenantID, model.ErrForbidden); err != nil {
		return output.RoleLister{}, err
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.RoleLister{}, err
	}
	got, err := s.roles.List(ctx, in.TenantID)
	return output.RoleLister{Items: mapOutput(got, roleOutput)}, err
}
