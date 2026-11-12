package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type projectRenamer struct {
	projects gateway.ProjectGateway
	authz    usecase.Authorizer
}

func NewProjectRenamer(projects gateway.ProjectGateway, authz usecase.Authorizer) usecase.ProjectRenamer {
	return &projectRenamer{projects: projects, authz: authz}
}

// Rename needs can_manage_projects on the tenant (or the platform). The tuples do not carry the name.
func (s *projectRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Project, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_projects", tenantID, entity.ErrForbidden); err != nil {
		return entity.Project{}, err
	}
	name, err := entity.CleanName("name", name)
	if err != nil {
		return entity.Project{}, err
	}
	if _, err := loadProject(ctx, s.projects, tenantID, id); err != nil {
		return entity.Project{}, err
	}
	return s.projects.Rename(ctx, id, name)
}
