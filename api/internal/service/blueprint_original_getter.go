package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintOriginalGetter struct {
	blueprints gateway.BlueprintGateway
	authz      usecase.Authorizer
}

func NewBlueprintOriginalGetter(blueprints gateway.BlueprintGateway, authz usecase.Authorizer) usecase.BlueprintOriginalGetter {
	return &blueprintOriginalGetter{blueprints: blueprints, authz: authz}
}

// GetOriginal requires can_edit, not can_view: a CAD original can hold more than the extracted plan (other
// layers, title-block text), so viewers get the plan and not the file. The file belongs to the blueprint, not
// to every version: version 0 means the newest stored file (the import, or the conversion), not the latest edit.
func (s *blueprintOriginalGetter) GetOriginal(ctx context.Context, actor, tenantID, id string, version int) (model.BlueprintOriginal, error) {
	_, err := loadBlueprint(ctx, s.blueprints, tenantID, id)
	if err != nil {
		return model.BlueprintOriginal{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", model.ObjectTypeBlueprint, id); err != nil {
		return model.BlueprintOriginal{}, err
	}
	if version == 0 {
		return s.blueprints.FindOriginal(ctx, id)
	}
	return s.blueprints.GetOriginal(ctx, id, version)
}
