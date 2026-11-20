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

type blueprintLister struct {
	blueprints gateway.BlueprintGateway
	authz      helper.Authorizer
}

func NewBlueprintLister(blueprints gateway.BlueprintGateway, authz helper.Authorizer) usecase.BlueprintLister {
	return &blueprintLister{blueprints: blueprints, authz: authz}
}

// List is ListObjects can_view, then the tenant's rows with those ids.
func (s *blueprintLister) List(ctx context.Context, in input.BlueprintLister) (output.BlueprintLister, error) {
	if in.ProjectID != "" {
		if err := model.RequireUUID("project_id", in.ProjectID); err != nil {
			return output.BlueprintLister{}, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, in.Actor, "can_view", model.ObjectTypeBlueprint)
	if err != nil || len(ids) == 0 {
		return output.BlueprintLister{}, err
	}
	blueprints, err := s.blueprints.ListByIDs(ctx, in.TenantID, ids, in.ProjectID)
	if err != nil {
		return output.BlueprintLister{}, err
	}
	got, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeBlueprint, blueprints, blueprintID)
	return output.BlueprintLister{Items: mapVisibleOutput(got, blueprintOutput)}, err
}
