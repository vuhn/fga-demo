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

type blueprintBriefUpdater struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      helper.Authorizer
}

func NewBlueprintBriefUpdater(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz helper.Authorizer) usecase.BlueprintBriefUpdater {
	return &blueprintBriefUpdater{blueprints: blueprints, engine: engine, authz: authz}
}

// UpdateBrief is the form edit: can_edit and a current base_version. The
// engine rebuilds without repair, so the user's numbers are never changed
// silently; a brief that cannot be built is 400.
func (s *blueprintBriefUpdater) UpdateBrief(ctx context.Context, in input.BlueprintBriefUpdater) (output.BlueprintBriefUpdater, error) {
	if err := validBrief(in.Brief); err != nil {
		return output.BlueprintBriefUpdater{}, err
	}
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID, in.BaseVersion)
	if err != nil {
		return output.BlueprintBriefUpdater{}, err
	}
	if err := requireBrief(cur); err != nil {
		return output.BlueprintBriefUpdater{}, err
	}
	out, err := s.engine.Generate(ctx, in.Brief, cur.Model, false)
	if err != nil {
		return output.BlueprintBriefUpdater{}, err
	}
	if err := requireBuilt(out); err != nil {
		return output.BlueprintBriefUpdater{}, err
	}
	got, err := appendBlueprintVersion(ctx, s.blueprints, b, outcomeVersion(in.ID, model.BlueprintVersionEdit, "edited the brief", in.Actor, out), in.BaseVersion)
	return output.BlueprintBriefUpdater{BlueprintDetail: blueprintDetailOutput(got)}, err
}
