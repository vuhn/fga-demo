package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintPlanEditor struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
}

func NewBlueprintPlanEditor(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintPlanEditor {
	return &blueprintPlanEditor{blueprints: blueprints, engine: engine, authz: authz}
}

const maxPlanEditBytes = 64 << 10

// Edit requires can_edit and a current base_version, then asks the engine to apply the explicit ops (no LLM) to
// the model as drawn. The Check and the version test run before the engine; the single write comes after it.
// Only an imported plan can be edited this way: a generated one is edited through its brief.
func (s *blueprintPlanEditor) Edit(ctx context.Context, actor, tenantID, id string, ops json.RawMessage, baseVersion int) (usecase.BlueprintDetail, error) {
	if len(ops) == 0 || len(ops) > maxPlanEditBytes || ops[0] != '{' || !json.Valid(ops) {
		return usecase.BlueprintDetail{}, model.Invalid("ops must be a JSON object of at most %d bytes", maxPlanEditBytes)
	}
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, actor, tenantID, id, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if !isImported(cur) {
		return usecase.BlueprintDetail{}, model.Invalid("only an imported plan is edited directly; edit this blueprint through its brief")
	}
	out, err := s.engine.EditPlan(ctx, cur.Model, ops)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	msg := "edited"
	if len(out.Notes) > 0 {
		msg = strings.TrimSpace(out.Notes[0])
	}
	if r := []rune(msg); len(r) > 500 {
		msg = string(r[:500])
	}
	return appendBlueprintVersion(ctx, s.blueprints, b, planOutcomeVersion(id, model.BlueprintVersionEdit, msg, actor, out), baseVersion)
}
