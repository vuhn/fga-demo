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

type roleUpdater struct {
	roles gateway.RoleGateway
	authz helper.Authorizer
}

func NewRoleUpdater(roles gateway.RoleGateway, authz helper.Authorizer) usecase.RoleUpdater {
	return &roleUpdater{roles: roles, authz: authz}
}

// UpdateName changes the display name. The tuples do not carry the name.
func (s *roleUpdater) UpdateName(ctx context.Context, in input.RoleUpdater) (output.RoleUpdater, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.RoleUpdater{}, err
	}
	name, err := model.CleanRoleName(in.Name)
	if err != nil {
		return output.RoleUpdater{}, err
	}
	role, err := s.roles.Get(ctx, in.TenantID, in.RoleID)
	if err != nil {
		return output.RoleUpdater{}, err
	}
	if role.Name == name {
		return output.RoleUpdater{Role: roleOutput(role)}, nil
	}
	if err := s.roles.UpdateName(ctx, in.TenantID, in.RoleID, name); err != nil {
		return output.RoleUpdater{}, err
	}
	role.Name = name
	return output.RoleUpdater{Role: roleOutput(role)}, nil
}
