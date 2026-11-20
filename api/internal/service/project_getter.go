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

type projectGetter struct {
	projects gateway.ProjectGateway
	authz    helper.Authorizer
}

func NewProjectGetter(projects gateway.ProjectGateway, authz helper.Authorizer) usecase.ProjectGetter {
	return &projectGetter{projects: projects, authz: authz}
}

// Get is 404 when the actor cannot view the project.
func (s *projectGetter) Get(ctx context.Context, in input.ProjectGetter) (output.ProjectGetter, error) {
	p, err := loadProject(ctx, s.projects, in.TenantID, in.ID)
	if err != nil {
		return output.ProjectGetter{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeProject, in.ID); err != nil {
		return output.ProjectGetter{}, err
	}
	vs, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeProject, []model.Project{p}, projectID)
	if err != nil {
		return output.ProjectGetter{}, err
	}
	return output.ProjectGetter{Visible: visibleOutput(vs[0], projectOutput)}, nil
}
