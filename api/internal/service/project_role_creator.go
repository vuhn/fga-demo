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

type projectRoleCreator struct {
	projects  gateway.ProjectGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectRoleRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewProjectRoleCreator(projects gateway.ProjectGateway, roles gateway.ProjectRoleGateway, relations gateway.ProjectRoleRelationGateway, authz helper.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.ProjectRoleCreator {
	return &projectRoleCreator{projects: projects, roles: roles, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create adds an empty project role. Platform only, like tenant custom roles.
func (s *projectRoleCreator) Create(ctx context.Context, in input.ProjectRoleCreator) (output.ProjectRoleCreator, error) {
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.ProjectRoleCreator{}, err
	}
	name, err := model.CleanRoleName(in.Name)
	if err != nil {
		return output.ProjectRoleCreator{}, err
	}
	if in.Affiliation != model.AffiliationInternal && in.Affiliation != model.AffiliationExternal {
		return output.ProjectRoleCreator{}, model.Invalid("unknown affiliation %q", in.Affiliation)
	}
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return output.ProjectRoleCreator{}, err
	}
	role := model.ProjectRole{ID: s.ids.NewID(), TenantID: in.TenantID, ProjectID: in.ProjectID, Name: name, Affiliation: in.Affiliation, Permissions: []string{}, Assignees: []model.RoleAssignee{}}
	tuples, err := s.relations.Created(in.ProjectID, role.ID, in.Affiliation)
	if err != nil {
		return output.ProjectRoleCreator{}, err
	}
	if err := s.roles.Insert(ctx, role); err != nil {
		return output.ProjectRoleCreator{}, err
	}
	err = s.guard.Grant(ctx, tuples, nil, func(ctx context.Context) error {
		return s.roles.Delete(ctx, in.ProjectID, role.ID)
	})
	if err != nil {
		return output.ProjectRoleCreator{}, err
	}
	got, err := s.roles.Get(ctx, in.ProjectID, role.ID)
	return output.ProjectRoleCreator{ProjectRole: projectRoleOutput(got)}, err
}
