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

type roleGetter struct {
	roles gateway.RoleGateway
	authz helper.Authorizer
}

func NewRoleGetter(roles gateway.RoleGateway, authz helper.Authorizer) usecase.RoleGetter {
	return &roleGetter{roles: roles, authz: authz}
}

func (s *roleGetter) Get(ctx context.Context, in input.RoleGetter) (output.RoleGetter, error) {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_members", in.TenantID, model.ErrForbidden); err != nil {
		return output.RoleGetter{}, err
	}
	got, err := s.roles.Get(ctx, in.TenantID, in.RoleID)
	return output.RoleGetter{Role: roleOutput(got)}, err
}
