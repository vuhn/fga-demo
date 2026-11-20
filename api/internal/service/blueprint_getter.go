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

type blueprintGetter struct {
	blueprints gateway.BlueprintGateway
	authz      helper.Authorizer
}

func NewBlueprintGetter(blueprints gateway.BlueprintGateway, authz helper.Authorizer) usecase.BlueprintGetter {
	return &blueprintGetter{blueprints: blueprints, authz: authz}
}

// Get returns the blueprint and its current version. It is 404 when the
// caller cannot view it.
func (s *blueprintGetter) Get(ctx context.Context, in input.BlueprintGetter) (output.BlueprintGetter, error) {
	b, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.ID)
	if err != nil {
		return output.BlueprintGetter{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeBlueprint, in.ID); err != nil {
		return output.BlueprintGetter{}, err
	}
	vs, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeBlueprint, []model.Blueprint{b}, blueprintID)
	if err != nil {
		return output.BlueprintGetter{}, err
	}
	v, err := s.blueprints.GetVersion(ctx, in.ID, b.CurrentVersion)
	if err != nil {
		return output.BlueprintGetter{}, err
	}
	return output.BlueprintGetter{BlueprintDetail: blueprintDetailOutput(blueprintDetail{Blueprint: b, Perms: vs[0].Perms, Version: v})}, nil
}
