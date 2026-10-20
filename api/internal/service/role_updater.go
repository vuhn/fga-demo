package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type roleUpdater struct {
	roles gateway.RoleGateway
	authz usecase.Authorizer
}

func NewRoleUpdater(roles gateway.RoleGateway, authz usecase.Authorizer) usecase.RoleUpdater {
	return &roleUpdater{roles: roles, authz: authz}
}

// UpdateName changes the display name. The tuples do not carry the name.
func (s *roleUpdater) UpdateName(ctx context.Context, actor, tenantID, roleID, name string) (entity.Role, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return entity.Role{}, err
	}
	name, err := entity.CleanRoleName(name)
	if err != nil {
		return entity.Role{}, err
	}
	role, err := s.roles.Get(ctx, tenantID, roleID)
	if err != nil {
		return entity.Role{}, err
	}
	if role.Name == name {
		return role, nil
	}
	if err := s.roles.UpdateName(ctx, tenantID, roleID, name); err != nil {
		return entity.Role{}, err
	}
	role.Name = name
	return role, nil
}
