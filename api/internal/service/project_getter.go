package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type projectGetter struct {
	projects gateway.ProjectGateway
	authz    usecase.Authorizer
}

func NewProjectGetter(projects gateway.ProjectGateway, authz usecase.Authorizer) usecase.ProjectGetter {
	return &projectGetter{projects: projects, authz: authz}
}

// Get is 404 when the actor cannot view the project.
func (s *projectGetter) Get(ctx context.Context, actor, tenantID, id string) (model.Visible[model.Project], error) {
	p, err := loadProject(ctx, s.projects, tenantID, id)
	if err != nil {
		return model.Visible[model.Project]{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", model.ObjectTypeProject, id); err != nil {
		return model.Visible[model.Project]{}, err
	}
	vs, err := withPerms(ctx, s.authz, actor, model.ObjectTypeProject, []model.Project{p}, projectID)
	if err != nil {
		return model.Visible[model.Project]{}, err
	}
	return vs[0], nil
}
