package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintGetter struct {
	blueprints gateway.BlueprintGateway
	authz      usecase.Authorizer
}

func NewBlueprintGetter(blueprints gateway.BlueprintGateway, authz usecase.Authorizer) usecase.BlueprintGetter {
	return &blueprintGetter{blueprints: blueprints, authz: authz}
}

// Get returns the blueprint and its current version. It is 404 when the
// caller cannot view it.
func (s *blueprintGetter) Get(ctx context.Context, actor, tenantID, id string) (usecase.BlueprintDetail, error) {
	b, err := loadBlueprint(ctx, s.blueprints, tenantID, id)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", entity.ObjectTypeBlueprint, id); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	vs, err := withPerms(ctx, s.authz, actor, entity.ObjectTypeBlueprint, []entity.Blueprint{b}, blueprintID)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	v, err := s.blueprints.GetVersion(ctx, id, b.CurrentVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	return usecase.BlueprintDetail{Blueprint: b, Perms: vs[0].Perms, Version: v}, nil
}
