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

type projectRolePermissionSetter struct {
	projects  gateway.ProjectGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectRoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewProjectRolePermissionSetter(projects gateway.ProjectGateway, roles gateway.ProjectRoleGateway, relations gateway.ProjectRoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.ProjectRolePermissionSetter {
	return &projectRolePermissionSetter{projects: projects, roles: roles, relations: relations, authz: authz, guard: guard}
}

// SetPermissions replaces the role's catalog. Removals go to OpenFGA first;
// additions are written to Postgres first. A failure in the middle leaves
// fewer grants, never more. Platform only.
func (s *projectRolePermissionSetter) SetPermissions(ctx context.Context, in input.ProjectRolePermissionSetter) (output.ProjectRolePermissionSetter, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.ProjectRolePermissionSetter{}, err
	}
	next, err := model.NormalizePermissions(in.Permissions)
	if err != nil {
		return output.ProjectRolePermissionSetter{}, err
	}
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return output.ProjectRolePermissionSetter{}, err
	}
	role, err := s.roles.Get(ctx, in.ProjectID, in.RoleID)
	if err != nil {
		return output.ProjectRolePermissionSetter{}, err
	}
	revoke, grant := permissionDiff(role.Permissions, next)
	if len(revoke) > 0 {
		tuples, err := s.relations.Permissions(in.ProjectID, in.RoleID, revoke)
		if err != nil {
			return output.ProjectRolePermissionSetter{}, err
		}
		err = s.guard.Revoke(ctx, tuples, tuples, func(ctx context.Context) error {
			return s.roles.DeletePermissions(ctx, in.RoleID, revoke)
		})
		if err != nil {
			return output.ProjectRolePermissionSetter{}, err
		}
	}
	if len(grant) > 0 {
		tuples, err := s.relations.Permissions(in.ProjectID, in.RoleID, grant)
		if err != nil {
			return output.ProjectRolePermissionSetter{}, err
		}
		if err := s.roles.InsertPermissions(ctx, in.RoleID, grant); err != nil {
			return output.ProjectRolePermissionSetter{}, err
		}
		err = s.guard.Grant(ctx, tuples, nil, func(ctx context.Context) error {
			return s.roles.DeletePermissions(ctx, in.RoleID, grant)
		})
		if err != nil {
			return output.ProjectRolePermissionSetter{}, err
		}
	}
	got, err := s.roles.Get(ctx, in.ProjectID, in.RoleID)
	return output.ProjectRolePermissionSetter{ProjectRole: projectRoleOutput(got)}, err
}
