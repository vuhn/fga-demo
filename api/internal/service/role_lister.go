package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type roleLister struct {
	tenants gateway.TenantGateway
	roles   gateway.RoleGateway
	authz   usecase.Authorizer
}

func NewRoleLister(tenants gateway.TenantGateway, roles gateway.RoleGateway, authz usecase.Authorizer) usecase.RoleLister {
	return &roleLister{tenants: tenants, roles: roles, authz: authz}
}

func (s *roleLister) List(ctx context.Context, actor, tenantID string) ([]entity.Role, error) {
	// Member managers need the list to offer custom roles next to the built-in ones.
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return nil, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return nil, err
	}
	return s.roles.List(ctx, tenantID)
}
