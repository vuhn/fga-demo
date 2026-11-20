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

type rolePermissionSetter struct {
	roles     gateway.RoleGateway
	relations gateway.RoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewRolePermissionSetter(roles gateway.RoleGateway, relations gateway.RoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.RolePermissionSetter {
	return &rolePermissionSetter{roles: roles, relations: relations, authz: authz, guard: guard}
}

// SetPermissions replaces the role's catalog. Removals go to OpenFGA first;
// additions are written to Postgres first. A failure in the middle leaves
// fewer grants, never more.
func (s *rolePermissionSetter) SetPermissions(ctx context.Context, in input.RolePermissionSetter) (output.RolePermissionSetter, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.RolePermissionSetter{}, err
	}
	next, err := model.NormalizePermissions(in.Permissions)
	if err != nil {
		return output.RolePermissionSetter{}, err
	}
	role, err := s.roles.Get(ctx, in.TenantID, in.RoleID)
	if err != nil {
		return output.RolePermissionSetter{}, err
	}
	revoke, grant := permissionDiff(role.Permissions, next)
	if len(revoke) > 0 {
		tuples, err := s.relations.Permissions(in.TenantID, in.RoleID, revoke)
		if err != nil {
			return output.RolePermissionSetter{}, err
		}
		err = s.guard.Revoke(ctx, tuples, tuples, func(ctx context.Context) error {
			return s.roles.DeletePermissions(ctx, in.RoleID, revoke)
		})
		if err != nil {
			return output.RolePermissionSetter{}, err
		}
	}
	if len(grant) > 0 {
		tuples, err := s.relations.Permissions(in.TenantID, in.RoleID, grant)
		if err != nil {
			return output.RolePermissionSetter{}, err
		}
		if err := s.roles.InsertPermissions(ctx, in.RoleID, grant); err != nil {
			return output.RolePermissionSetter{}, err
		}
		err = s.guard.Grant(ctx, tuples, nil, func(ctx context.Context) error {
			return s.roles.DeletePermissions(ctx, in.RoleID, grant)
		})
		if err != nil {
			return output.RolePermissionSetter{}, err
		}
	}
	got, err := s.roles.Get(ctx, in.TenantID, in.RoleID)
	return output.RolePermissionSetter{Role: roleOutput(got)}, err
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
	for _, p := range model.CustomPermissions {
		switch {
		case have[p] && !want[p]:
			revoke = append(revoke, p)
		case want[p] && !have[p]:
			grant = append(grant, p)
		}
	}
	return revoke, grant
}
