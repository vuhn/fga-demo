package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type blueprintDeleter struct {
	blueprints gateway.BlueprintGateway
	authz      usecase.Authorizer
	guard      helper.WriteGuard
}

func NewBlueprintDeleter(blueprints gateway.BlueprintGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.BlueprintDeleter {
	return &blueprintDeleter{blueprints: blueprints, authz: authz, guard: guard}
}

// Delete requires can_delete. It removes every tuple on the blueprint, then
// the row and its grants.
func (s *blueprintDeleter) Delete(ctx context.Context, actor, tenantID, id string) error {
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, id); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, actor, "can_delete", entity.ObjectTypeBlueprint, id); err != nil {
		return err
	}
	return s.guard.RevokeObject(ctx, entity.ObjectTypeBlueprint, id, func(ctx context.Context) error {
		return s.blueprints.Delete(ctx, id)
	})
}
