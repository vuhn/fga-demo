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

type projectRoleDeleter struct {
	projects  gateway.ProjectGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectRoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewProjectRoleDeleter(projects gateway.ProjectGateway, roles gateway.ProjectRoleGateway, relations gateway.ProjectRoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.ProjectRoleDeleter {
	return &projectRoleDeleter{projects: projects, roles: roles, relations: relations, authz: authz, guard: guard}
}

// Delete removes the role's tuples first, then the row. A role still held by
// project members cannot be deleted. Platform only.
func (s *projectRoleDeleter) Delete(ctx context.Context, in input.ProjectRoleDeleter) error {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return err
	}
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return err
	}
	role, err := s.roles.Get(ctx, in.ProjectID, in.RoleID)
	if err != nil {
		return err
	}
	if len(role.Assignees) > 0 {
		return fmt.Errorf("%w: role is assigned to project members", model.ErrConflict)
	}
	tuples, err := s.relations.Removal(model.RemoveProjectRoleParams{ProjectID: in.ProjectID, RoleID: in.RoleID, Affiliation: role.Affiliation, Permissions: role.Permissions})
	if err != nil {
		return err
	}
	return s.guard.Revoke(ctx, tuples, tuples, func(ctx context.Context) error {
		return s.roles.Delete(ctx, in.ProjectID, in.RoleID)
	})
}
