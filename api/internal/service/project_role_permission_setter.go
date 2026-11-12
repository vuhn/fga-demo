package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type projectRolePermissionSetter struct {
	projects  gateway.ProjectGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectRoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewProjectRolePermissionSetter(projects gateway.ProjectGateway, roles gateway.ProjectRoleGateway, relations gateway.ProjectRoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.ProjectRolePermissionSetter {
	return &projectRolePermissionSetter{projects: projects, roles: roles, relations: relations, authz: authz, guard: guard}
}

// SetPermissions replaces the role's catalog. Removals go to OpenFGA first;
// additions are written to Postgres first. A failure in the middle leaves
// fewer grants, never more. Platform only.
func (s *projectRolePermissionSetter) SetPermissions(ctx context.Context, actor, tenantID, projectID, roleID string, permissions []string) (entity.ProjectRole, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return entity.ProjectRole{}, err
	}
	next, err := entity.NormalizePermissions(permissions)
	if err != nil {
		return entity.ProjectRole{}, err
	}
	if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
		return entity.ProjectRole{}, err
	}
	role, err := s.roles.Get(ctx, projectID, roleID)
	if err != nil {
		return entity.ProjectRole{}, err
	}
	revoke, grant := permissionDiff(role.Permissions, next)
	if len(revoke) > 0 {
		tuples, err := s.relations.Permissions(projectID, roleID, revoke)
		if err != nil {
			return entity.ProjectRole{}, err
		}
		err = s.guard.Revoke(ctx, tuples, tuples, func(ctx context.Context) error {
			return s.roles.DeletePermissions(ctx, roleID, revoke)
		})
		if err != nil {
			return entity.ProjectRole{}, err
		}
	}
	if len(grant) > 0 {
		tuples, err := s.relations.Permissions(projectID, roleID, grant)
		if err != nil {
			return entity.ProjectRole{}, err
		}
		if err := s.roles.InsertPermissions(ctx, roleID, grant); err != nil {
			return entity.ProjectRole{}, err
		}
		err = s.guard.Grant(ctx, tuples, nil, func(ctx context.Context) error {
			return s.roles.DeletePermissions(ctx, roleID, grant)
		})
		if err != nil {
			return entity.ProjectRole{}, err
		}
	}
	return s.roles.Get(ctx, projectID, roleID)
}
