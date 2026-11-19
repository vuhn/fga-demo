package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
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
func (s *blueprintRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (model.Blueprint, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, id); err != nil {
		return model.Blueprint{}, err
	}
	name, err := model.CleanName("name", name)
	if err != nil {
		return model.Blueprint{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", model.ObjectTypeBlueprint, id); err != nil {
		return model.Blueprint{}, err
	}
	return s.blueprints.Rename(ctx, id, name)
}
