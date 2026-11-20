package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type blueprintPlanConverter struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      helper.Authorizer
}

func NewBlueprintPlanConverter(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz helper.Authorizer) usecase.BlueprintPlanConverter {
	return &blueprintPlanConverter{blueprints: blueprints, engine: engine, authz: authz}
}

// Convert requires can_edit and a current base_version, like every edit. The engine draws the stored plan into
// a DXF and reads it back (no LLM) before anything is written; the new version has no brief and the drawing is
// stored with it in one transaction. Older versions keep their briefs. Only a plan that still has a brief can be
// converted; a DXF plan is already one.
func (s *blueprintPlanConverter) Convert(ctx context.Context, in input.BlueprintPlanConverter) (output.BlueprintPlanConverter, error) {
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID, in.BaseVersion)
	if err != nil {
		return output.BlueprintPlanConverter{}, err
	}
	if isImported(cur) {
		return output.BlueprintPlanConverter{}, model.Invalid("this blueprint is already a DXF plan")
	}
	out, err := s.engine.DrawPlan(ctx, cur.Model, b.Name)
	if err != nil {
		return output.BlueprintPlanConverter{}, err
	}
	if err := requireBuilt(out); err != nil {
		return output.BlueprintPlanConverter{}, err
	}
	if len(out.DXF) == 0 {
		return output.BlueprintPlanConverter{}, model.Invalid("the engine returned a plan without its drawing")
	}
	v := planOutcomeVersion(in.ID, model.BlueprintVersionEdit, "converted to a DXF plan", in.Actor, out)
	sum := sha256.Sum256(out.DXF)
	orig := model.BlueprintOriginal{BlueprintID: in.ID, Version: in.BaseVersion + 1, Name: drawnFileName(b.Name), MimeType: model.DXFMimeType,
		ByteSize: int64(len(out.DXF)), SHA256: hex.EncodeToString(sum[:]), Data: out.DXF}
	if err := s.blueprints.AppendVersionWithOriginal(ctx, v, in.BaseVersion, orig); err != nil {
		return output.BlueprintPlanConverter{}, err
	}
	b, err = s.blueprints.Get(ctx, in.ID)
	if err != nil {
		return output.BlueprintPlanConverter{}, err
	}
	v, err = s.blueprints.GetVersion(ctx, in.ID, in.BaseVersion+1)
	return output.BlueprintPlanConverter{BlueprintDetail: blueprintDetailOutput(blueprintDetail{Blueprint: b, Version: v})}, err
}
