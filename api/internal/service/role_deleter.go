package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type roleDeleter struct {
	roles     gateway.RoleGateway
	relations gateway.RoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewRoleDeleter(roles gateway.RoleGateway, relations gateway.RoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.RoleDeleter {
	return &roleDeleter{roles: roles, relations: relations, authz: authz, guard: guard}
}

// Delete removes the role's tuples first, then the row. Permissions and
// assignees cascade in Postgres.
func (s *roleDeleter) Delete(ctx context.Context, in input.RoleDeleter) error {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return err
	}
	role, err := s.roles.Get(ctx, in.TenantID, in.RoleID)
	if err != nil {
		return err
	}
	if len(role.Assignees) > 0 {
		return fmt.Errorf("%w: role is assigned to members", model.ErrConflict)
	}
	tuples, err := s.relations.Removal(model.RemoveRoleParams{TenantID: in.TenantID, RoleID: in.RoleID, Affiliation: role.Affiliation, Permissions: role.Permissions})
	if err != nil {
		return err
	}
	return s.guard.Revoke(ctx, tuples, tuples, func(ctx context.Context) error {
		return s.roles.Delete(ctx, in.TenantID, in.RoleID)
	})
}
