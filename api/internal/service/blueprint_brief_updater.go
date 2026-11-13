package service

import (
	"context"
	"encoding/json"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintBriefUpdater struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
}

func NewBlueprintBriefUpdater(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintBriefUpdater {
	return &blueprintBriefUpdater{blueprints: blueprints, engine: engine, authz: authz}
}

// UpdateBrief is the form edit: can_edit and a current base_version. The
// engine rebuilds without repair, so the user's numbers are never changed
// silently; a brief that cannot be built is 400.
func (s *blueprintBriefUpdater) UpdateBrief(ctx context.Context, actor, tenantID, id string, brief json.RawMessage, baseVersion int) (usecase.BlueprintDetail, error) {
	if err := validBrief(brief); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, actor, tenantID, id, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	out, err := s.engine.Generate(ctx, brief, cur.Model, false)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	return appendBlueprintVersion(ctx, s.blueprints, b, outcomeVersion(id, entity.BlueprintVersionEdit, "edited the brief", actor, out), baseVersion)
}
