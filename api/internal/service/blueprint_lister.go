package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintLister struct {
	blueprints gateway.BlueprintGateway
	authz      usecase.Authorizer
}

func NewBlueprintLister(blueprints gateway.BlueprintGateway, authz usecase.Authorizer) usecase.BlueprintLister {
	return &blueprintLister{blueprints: blueprints, authz: authz}
}

// List is ListObjects can_view, then the tenant's rows with those ids.
func (s *blueprintLister) List(ctx context.Context, actor, tenantID, projectID string) ([]model.Visible[model.Blueprint], error) {
	if projectID != "" {
		if err := model.RequireUUID("project_id", projectID); err != nil {
			return nil, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_view", model.ObjectTypeBlueprint)
	if err != nil || len(ids) == 0 {
		return []model.Visible[model.Blueprint]{}, err
	}
	blueprints, err := s.blueprints.ListByIDs(ctx, tenantID, ids, projectID)
	if err != nil {
		return nil, err
	}
	return withPerms(ctx, s.authz, actor, model.ObjectTypeBlueprint, blueprints, blueprintID)
}
