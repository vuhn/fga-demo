package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
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
func (s *projectGetter) Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Project], error) {
	p, err := loadProject(ctx, s.projects, tenantID, id)
	if err != nil {
		return entity.Visible[entity.Project]{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", entity.ObjectTypeProject, id); err != nil {
		return entity.Visible[entity.Project]{}, err
	}
	vs, err := withPerms(ctx, s.authz, actor, entity.ObjectTypeProject, []entity.Project{p}, projectID)
	if err != nil {
		return entity.Visible[entity.Project]{}, err
	}
	return vs[0], nil
}
