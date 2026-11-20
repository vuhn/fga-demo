package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type blueprintPlanEditor struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      helper.Authorizer
}

func NewBlueprintPlanEditor(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz helper.Authorizer) usecase.BlueprintPlanEditor {
	return &blueprintPlanEditor{blueprints: blueprints, engine: engine, authz: authz}
}

const maxPlanEditBytes = 64 << 10

// Edit requires can_edit and a current base_version, then asks the engine to apply the explicit ops (no LLM) to
// the model as drawn. The Check and the version test run before the engine; the single write comes after it.
// Only an imported plan can be edited this way: a generated one is edited through its brief.
func (s *blueprintPlanEditor) Edit(ctx context.Context, in input.BlueprintPlanEditor) (output.BlueprintPlanEditor, error) {
	if len(in.Ops) == 0 || len(in.Ops) > maxPlanEditBytes || in.Ops[0] != '{' || !json.Valid(in.Ops) {
		return output.BlueprintPlanEditor{}, model.Invalid("ops must be a JSON object of at most %d bytes", maxPlanEditBytes)
	}
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID, in.BaseVersion)
	if err != nil {
		return output.BlueprintPlanEditor{}, err
	}
	if !isImported(cur) {
		return output.BlueprintPlanEditor{}, model.Invalid("only an imported plan is edited directly; edit this blueprint through its brief")
	}
	out, err := s.engine.EditPlan(ctx, cur.Model, in.Ops)
	if err != nil {
		return output.BlueprintPlanEditor{}, err
	}
	if err := requireBuilt(out); err != nil {
		return output.BlueprintPlanEditor{}, err
	}
	msg := "edited"
	if len(out.Notes) > 0 {
		msg = strings.TrimSpace(out.Notes[0])
	}
	if r := []rune(msg); len(r) > 500 {
		msg = string(r[:500])
	}
	got, err := appendBlueprintVersion(ctx, s.blueprints, b, planOutcomeVersion(in.ID, model.BlueprintVersionEdit, msg, in.Actor, out), in.BaseVersion)
	return output.BlueprintPlanEditor{BlueprintDetail: blueprintDetailOutput(got)}, err
}
