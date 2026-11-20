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

type blueprintOriginalGetter struct {
	blueprints gateway.BlueprintGateway
	authz      helper.Authorizer
}

func NewBlueprintOriginalGetter(blueprints gateway.BlueprintGateway, authz helper.Authorizer) usecase.BlueprintOriginalGetter {
	return &blueprintOriginalGetter{blueprints: blueprints, authz: authz}
}

// GetOriginal requires can_edit, not can_view: a CAD original can hold more than the extracted plan (other
// layers, title-block text), so viewers get the plan and not the file. The file belongs to the blueprint, not
// to every version: version 0 means the newest stored file (the import, or the conversion), not the latest edit.
func (s *blueprintOriginalGetter) GetOriginal(ctx context.Context, in input.BlueprintOriginalGetter) (output.BlueprintOriginalGetter, error) {
	_, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.ID)
	if err != nil {
		return output.BlueprintOriginalGetter{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_edit", model.ObjectTypeBlueprint, in.ID); err != nil {
		return output.BlueprintOriginalGetter{}, err
	}
	if in.Version == 0 {
		got, err := s.blueprints.FindOriginal(ctx, in.ID)
		return output.BlueprintOriginalGetter{BlueprintOriginal: blueprintOriginalOutput(got)}, err
	}
	got, err := s.blueprints.GetOriginal(ctx, in.ID, in.Version)
	return output.BlueprintOriginalGetter{BlueprintOriginal: blueprintOriginalOutput(got)}, err
}
