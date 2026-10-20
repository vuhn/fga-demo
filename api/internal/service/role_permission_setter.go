package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type rolePermissionSetter struct {
	roles     gateway.RoleGateway
	relations gateway.RoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewRolePermissionSetter(roles gateway.RoleGateway, relations gateway.RoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.RolePermissionSetter {
	return &rolePermissionSetter{roles: roles, relations: relations, authz: authz, guard: guard}
}

// SetPermissions replaces the role's catalog. Removals go to OpenFGA first;
// additions are written to Postgres first. A failure in the middle leaves
// fewer grants, never more.
func (s *rolePermissionSetter) SetPermissions(ctx context.Context, actor, tenantID, roleID string, permissions []string) (entity.Role, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return entity.Role{}, err
	}
	next, err := entity.NormalizePermissions(permissions)
	if err != nil {
		return entity.Role{}, err
	}
	role, err := s.roles.Get(ctx, tenantID, roleID)
	if err != nil {
		return entity.Role{}, err
	}
	revoke, grant := permissionDiff(role.Permissions, next)
	if len(revoke) > 0 {
		tuples, err := s.relations.Permissions(tenantID, roleID, revoke)
		if err != nil {
			return entity.Role{}, err
		}
		err = s.guard.Revoke(ctx, tuples, tuples, func(ctx context.Context) error {
			return s.roles.DeletePermissions(ctx, roleID, revoke)
		})
		if err != nil {
			return entity.Role{}, err
		}
	}
	if len(grant) > 0 {
		tuples, err := s.relations.Permissions(tenantID, roleID, grant)
		if err != nil {
			return entity.Role{}, err
		}
		if err := s.roles.InsertPermissions(ctx, roleID, grant); err != nil {
			return entity.Role{}, err
		}
		err = s.guard.Grant(ctx, tuples, nil, func(ctx context.Context) error {
			return s.roles.DeletePermissions(ctx, roleID, grant)
		})
		if err != nil {
			return entity.Role{}, err
		}
	}
	return s.roles.Get(ctx, tenantID, roleID)
}

func permissionDiff(old, next []string) (revoke, grant []string) {
	have := map[string]bool{}
	for _, p := range old {
		have[p] = true
	}
	want := map[string]bool{}
	for _, p := range next {
		want[p] = true
	}
	for _, p := range entity.CustomPermissions {
		switch {
		case have[p] && !want[p]:
			revoke = append(revoke, p)
		case want[p] && !have[p]:
			grant = append(grant, p)
		}
	}
	return revoke, grant
}
