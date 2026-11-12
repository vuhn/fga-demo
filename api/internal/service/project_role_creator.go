package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type projectRoleCreator struct {
	projects  gateway.ProjectGateway
	roles     gateway.ProjectRoleGateway
	relations gateway.ProjectRoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewProjectRoleCreator(projects gateway.ProjectGateway, roles gateway.ProjectRoleGateway, relations gateway.ProjectRoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.ProjectRoleCreator {
	return &projectRoleCreator{projects: projects, roles: roles, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create adds an empty project role. Platform only, like tenant custom roles.
func (s *projectRoleCreator) Create(ctx context.Context, actor, tenantID, projectID, name, affiliation string) (entity.ProjectRole, error) {
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return entity.ProjectRole{}, err
	}
	name, err := entity.CleanRoleName(name)
	if err != nil {
		return entity.ProjectRole{}, err
	}
	if affiliation != entity.AffiliationInternal && affiliation != entity.AffiliationExternal {
		return entity.ProjectRole{}, entity.Invalid("unknown affiliation %q", affiliation)
	}
	if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
		return entity.ProjectRole{}, err
	}
	role := entity.ProjectRole{ID: s.ids.NewID(), TenantID: tenantID, ProjectID: projectID, Name: name, Affiliation: affiliation, Permissions: []string{}, Assignees: []entity.RoleAssignee{}}
	tuples, err := s.relations.Created(projectID, role.ID, affiliation)
	if err != nil {
		return entity.ProjectRole{}, err
	}
	if err := s.roles.Insert(ctx, role); err != nil {
		return entity.ProjectRole{}, err
	}
	err = s.guard.Grant(ctx, tuples, nil, func(ctx context.Context) error {
		return s.roles.Delete(ctx, projectID, role.ID)
	})
	if err != nil {
		return entity.ProjectRole{}, err
	}
	return s.roles.Get(ctx, projectID, role.ID)
}
