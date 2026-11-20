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

type projectRenamer struct {
	projects gateway.ProjectGateway
	authz    helper.Authorizer
}

func NewProjectRenamer(projects gateway.ProjectGateway, authz helper.Authorizer) usecase.ProjectRenamer {
	return &projectRenamer{projects: projects, authz: authz}
}

// Rename needs can_manage_projects on the tenant (or the platform). The tuples do not carry the name.
func (s *projectRenamer) Rename(ctx context.Context, in input.ProjectRenamer) (output.ProjectRenamer, error) {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_projects", in.TenantID, model.ErrForbidden); err != nil {
		return output.ProjectRenamer{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.ProjectRenamer{}, err
	}
	if _, err := loadProject(ctx, s.projects, in.TenantID, in.ID); err != nil {
		return output.ProjectRenamer{}, err
	}
	got, err := s.projects.Rename(ctx, in.ID, name)
	return output.ProjectRenamer{Project: projectOutput(got)}, err
}
