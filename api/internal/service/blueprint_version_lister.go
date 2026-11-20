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

type blueprintVersionLister struct {
	blueprints gateway.BlueprintGateway
	authz      helper.Authorizer
}

func NewBlueprintVersionLister(blueprints gateway.BlueprintGateway, authz helper.Authorizer) usecase.BlueprintVersionLister {
	return &blueprintVersionLister{blueprints: blueprints, authz: authz}
}

// ListVersions requires can_view (404 when denied).
func (s *blueprintVersionLister) ListVersions(ctx context.Context, in input.BlueprintVersionLister) (output.BlueprintVersionLister, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.ID); err != nil {
		return output.BlueprintVersionLister{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeBlueprint, in.ID); err != nil {
		return output.BlueprintVersionLister{}, err
	}
	got, err := s.blueprints.ListVersions(ctx, in.ID)
	return output.BlueprintVersionLister{Items: mapOutput(got, blueprintVersionOutput)}, err
}
