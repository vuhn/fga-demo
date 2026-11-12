package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type projectRoleLister struct {
	projects gateway.ProjectGateway
	roles    gateway.ProjectRoleGateway
	authz    usecase.Authorizer
}

func NewProjectRoleLister(projects gateway.ProjectGateway, roles gateway.ProjectRoleGateway, authz usecase.Authorizer) usecase.ProjectRoleLister {
	return &projectRoleLister{projects: projects, roles: roles, authz: authz}
}

// List is for project member managers: they offer project roles next to the built-in ones.
func (s *projectRoleLister) List(ctx context.Context, actor, tenantID, projectID string) ([]entity.ProjectRole, error) {
	if _, err := loadProject(ctx, s.projects, tenantID, projectID); err != nil {
		return nil, err
	}
	if err := requireProjectManager(ctx, s.authz, actor, projectID); err != nil {
		return nil, err
	}
	return s.roles.List(ctx, projectID)
}
