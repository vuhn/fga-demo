package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintVersionLister struct {
	blueprints gateway.BlueprintGateway
	authz      usecase.Authorizer
}

func NewBlueprintVersionLister(blueprints gateway.BlueprintGateway, authz usecase.Authorizer) usecase.BlueprintVersionLister {
	return &blueprintVersionLister{blueprints: blueprints, authz: authz}
}

// ListVersions requires can_view (404 when denied).
func (s *blueprintVersionLister) ListVersions(ctx context.Context, actor, tenantID, id string) ([]entity.BlueprintVersion, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, id); err != nil {
		return nil, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", entity.ObjectTypeBlueprint, id); err != nil {
		return nil, err
	}
	return s.blueprints.ListVersions(ctx, id)
}
