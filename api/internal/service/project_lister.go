package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type projectLister struct {
	projects gateway.ProjectGateway
	authz    usecase.Authorizer
}

func NewProjectLister(projects gateway.ProjectGateway, authz usecase.Authorizer) usecase.ProjectLister {
	return &projectLister{projects: projects, authz: authz}
}

// List is ListObjects can_view, then a Postgres load of those ids in the tenant.
func (s *projectLister) List(ctx context.Context, actor, tenantID string) ([]entity.Visible[entity.Project], error) {
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_view", entity.ObjectTypeProject)
	if err != nil || len(ids) == 0 {
		return []entity.Visible[entity.Project]{}, err
	}
	ps, err := s.projects.ListByIDs(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return withPerms(ctx, s.authz, actor, entity.ObjectTypeProject, ps, projectID)
}
