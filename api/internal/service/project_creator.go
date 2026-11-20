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

type projectCreator struct {
	tenants   gateway.TenantGateway
	projects  gateway.ProjectGateway
	relations gateway.ProjectRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
	ids       gateway.IDGenerator
}

func NewProjectCreator(tenants gateway.TenantGateway, projects gateway.ProjectGateway, relations gateway.ProjectRelationGateway, authz helper.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.ProjectCreator {
	return &projectCreator{tenants: tenants, projects: projects, relations: relations, authz: authz, guard: guard, ids: ids}
}

// Create needs can_manage_projects on the tenant (or the platform). The
// creator gets no special grant: tenant roles decide, then project roles add.
func (s *projectCreator) Create(ctx context.Context, in input.ProjectCreator) (output.ProjectCreator, error) {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_manage_projects", in.TenantID, model.ErrForbidden); err != nil {
		return output.ProjectCreator{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.ProjectCreator{}, err
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.ProjectCreator{}, err
	}
	p := model.Project{ID: s.ids.NewID(), TenantID: in.TenantID, Name: name, CreatedBy: in.Actor}
	if err := s.projects.Create(ctx, p); err != nil {
		return output.ProjectCreator{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(p), nil, func(ctx context.Context) error {
		return s.projects.Delete(ctx, p.ID)
	})
	if err != nil {
		return output.ProjectCreator{}, err
	}
	got, err := s.projects.Get(ctx, p.ID)
	return output.ProjectCreator{Project: projectOutput(got)}, err
}
