package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintRenamer struct {
	blueprints gateway.BlueprintGateway
	authz      usecase.Authorizer
}

func NewBlueprintRenamer(blueprints gateway.BlueprintGateway, authz usecase.Authorizer) usecase.BlueprintRenamer {
	return &blueprintRenamer{blueprints: blueprints, authz: authz}
}

// Rename requires can_edit.
func (s *blueprintRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Blueprint, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, id); err != nil {
		return entity.Blueprint{}, err
	}
	name, err := entity.CleanName("name", name)
	if err != nil {
		return entity.Blueprint{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", entity.ObjectTypeBlueprint, id); err != nil {
		return entity.Blueprint{}, err
	}
	return s.blueprints.Rename(ctx, id, name)
}
