package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type roleDeleter struct {
	roles     gateway.RoleGateway
	relations gateway.RoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewRoleDeleter(roles gateway.RoleGateway, relations gateway.RoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.RoleDeleter {
	return &roleDeleter{roles: roles, relations: relations, authz: authz, guard: guard}
}

// Delete removes the role's tuples first, then the row. Permissions and
// assignees cascade in Postgres.
func (s *roleDeleter) Delete(ctx context.Context, actor, tenantID, roleID string) error {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return err
	}
	role, err := s.roles.Get(ctx, tenantID, roleID)
	if err != nil {
		return err
	}
	if len(role.Assignees) > 0 {
		return fmt.Errorf("%w: role is assigned to members", entity.ErrConflict)
	}
	tuples, err := s.relations.Removal(tenantID, roleID, role.Affiliation, role.Permissions, nil)
	if err != nil {
		return err
	}
	return s.guard.Revoke(ctx, tuples, tuples, func(ctx context.Context) error {
		return s.roles.Delete(ctx, tenantID, roleID)
	})
}
