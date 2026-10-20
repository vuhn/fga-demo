package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type roleGetter struct {
	roles gateway.RoleGateway
	authz usecase.Authorizer
}

func NewRoleGetter(roles gateway.RoleGateway, authz usecase.Authorizer) usecase.RoleGetter {
	return &roleGetter{roles: roles, authz: authz}
}

func (s *roleGetter) Get(ctx context.Context, actor, tenantID, roleID string) (entity.Role, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return entity.Role{}, err
	}
	return s.roles.Get(ctx, tenantID, roleID)
}
