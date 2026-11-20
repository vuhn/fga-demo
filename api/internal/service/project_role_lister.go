package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type projectRoleLister struct {
	projects gateway.ProjectGateway
	roles    gateway.ProjectRoleGateway
	authz    helper.Authorizer
}

func NewProjectRoleLister(projects gateway.ProjectGateway, roles gateway.ProjectRoleGateway, authz helper.Authorizer) usecase.ProjectRoleLister {
	return &projectRoleLister{projects: projects, roles: roles, authz: authz}
}

// List is for project member managers: they offer project roles next to the built-in ones.
func (s *projectRoleLister) List(ctx context.Context, in input.ProjectRoleLister) (output.ProjectRoleLister, error) {
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ProjectID); err != nil {
		return output.ProjectRoleLister{}, err
	}
	if err := requireProjectManager(ctx, s.authz, in.Actor, in.ProjectID); err != nil {
		return output.ProjectRoleLister{}, err
	}
	got, err := s.roles.List(ctx, in.ProjectID)
	return output.ProjectRoleLister{Items: mapOutput(got, projectRoleOutput)}, err
}
