package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// loadProject returns the project when it exists in the header tenant.
func loadProject(ctx context.Context, projects gateway.ProjectGateway, tenantID, id string) (entity.Project, error) {
	if err := entity.RequireUUID("project_id", id); err != nil {
		return entity.Project{}, err
	}
	p, err := projects.Get(ctx, id)
	if err != nil {
		return entity.Project{}, err
	}
	if p.TenantID != tenantID {
		return entity.Project{}, entity.ErrNotFound
	}
	return p, nil
}

// requireProjectManager is "platform admin OR project member manager": two
// Checks composed here, never a model relation that makes platform admins
// project members. can_manage_members on the project already includes the
// tenant's member managers.
func requireProjectManager(ctx context.Context, authz usecase.Authorizer, actor, projectID string) error {
	ok, err := authz.IsPlatformAdmin(ctx, actor)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return authz.Require(ctx, actor, "can_manage_members", entity.ObjectTypeProject, projectID)
}

func projectID(p entity.Project) string { return p.ID }
