package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintPlanConverter struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
}

func NewBlueprintPlanConverter(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintPlanConverter {
	return &blueprintPlanConverter{blueprints: blueprints, engine: engine, authz: authz}
}

// Convert requires can_edit and a current base_version, like every edit. The engine draws the stored plan into
// a DXF and reads it back (no LLM) before anything is written; the new version has no brief and the drawing is
// stored with it in one transaction. Older versions keep their briefs. Only a plan that still has a brief can be
// converted; a DXF plan is already one.
func (s *blueprintPlanConverter) Convert(ctx context.Context, actor, tenantID, id string, baseVersion int) (usecase.BlueprintDetail, error) {
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, actor, tenantID, id, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if isImported(cur) {
		return usecase.BlueprintDetail{}, model.Invalid("this blueprint is already a DXF plan")
	}
	out, err := s.engine.DrawPlan(ctx, cur.Model, b.Name)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if len(out.DXF) == 0 {
		return usecase.BlueprintDetail{}, model.Invalid("the engine returned a plan without its drawing")
	}
	v := planOutcomeVersion(id, model.BlueprintVersionEdit, "converted to a DXF plan", actor, out)
	sum := sha256.Sum256(out.DXF)
	orig := model.BlueprintOriginal{BlueprintID: id, Version: baseVersion + 1, Name: drawnFileName(b.Name), MimeType: model.DXFMimeType,
		ByteSize: int64(len(out.DXF)), SHA256: hex.EncodeToString(sum[:]), Data: out.DXF}
	if err := s.blueprints.AppendVersionWithOriginal(ctx, v, baseVersion, orig); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	b, err = s.blueprints.Get(ctx, id)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	v, err = s.blueprints.GetVersion(ctx, id, baseVersion+1)
	return usecase.BlueprintDetail{Blueprint: b, Version: v}, err
}
