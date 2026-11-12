package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type projectCreator struct {
	tenants   gateway.TenantGateway
	projects  gateway.ProjectGateway
	relations gateway.ProjectRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewProjectCreator(tenants gateway.TenantGateway, projects gateway.ProjectGateway, relations gateway.ProjectRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.ProjectCreator {
	return &projectCreator{tenants: tenants, projects: projects, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create needs can_manage_projects on the tenant (or the platform). The
// creator gets no special grant: tenant roles decide, then project roles add.
func (s *projectCreator) Create(ctx context.Context, actor, tenantID, name string) (entity.Project, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_projects", tenantID, entity.ErrForbidden); err != nil {
		return entity.Project{}, err
	}
	name, err := entity.CleanName("name", name)
	if err != nil {
		return entity.Project{}, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return entity.Project{}, err
	}
	p := entity.Project{ID: s.ids.NewID(), TenantID: tenantID, Name: name, CreatedBy: actor}
	if err := s.projects.Create(ctx, p); err != nil {
		return entity.Project{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(p), nil, func(ctx context.Context) error {
		return s.projects.Delete(ctx, p.ID)
	})
	if err != nil {
		return entity.Project{}, err
	}
	return s.projects.Get(ctx, p.ID)
}
