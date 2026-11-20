package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type blueprintCreator struct {
	projects   gateway.ProjectGateway
	blueprints gateway.BlueprintGateway
	relations  gateway.BlueprintRelationGateway
	engine     gateway.BlueprintEngine
	authz      helper.Authorizer
	guard      helper.WriteGuard
	ids        gateway.IDGenerator
}

func NewBlueprintCreator(projects gateway.ProjectGateway, blueprints gateway.BlueprintGateway, relations gateway.BlueprintRelationGateway,
	engine gateway.BlueprintEngine, authz helper.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.BlueprintCreator {
	return &blueprintCreator{projects: projects, blueprints: blueprints, relations: relations, engine: engine, authz: authz, guard: guard, ids: ids}
}

// Create requires can_create_blueprint on the project. The engine builds the
// brief (shrinking rooms when it is too big) into a plan and the DXF it drew
// before anything is written; a brief that still cannot be built is 400. The
// brief is only the input: the blueprint is stored like an import (version 1
// without a brief, the drawn DXF as its original) in one transaction. Then
// OpenFGA; a failure there deletes the Postgres rows.
func (s *blueprintCreator) Create(ctx context.Context, in input.BlueprintCreator) (output.BlueprintCreator, error) {
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.BlueprintCreator{}, err
	}
	if err := validBrief(in.Brief); err != nil {
		return output.BlueprintCreator{}, err
	}
	err = checkSourceKind(in.SourceKind)
	if err != nil {
		return output.BlueprintCreator{}, err
	}
	prompt := strings.TrimSpace(in.Prompt)
	if len([]rune(prompt)) > 2000 {
		return output.BlueprintCreator{}, model.Invalid("prompt must be at most 2000 characters")
	}
	if err := requireBlueprintCreate(ctx, s.projects, s.authz, in.Actor, in.TenantID, in.ProjectID); err != nil {
		return output.BlueprintCreator{}, err
	}
	out, err := s.engine.CreatePlan(ctx, in.Brief)
	if err != nil {
		return output.BlueprintCreator{}, err
	}
	if err := requireBuilt(out); err != nil {
		return output.BlueprintCreator{}, err
	}
	if len(out.DXF) == 0 {
		return output.BlueprintCreator{}, model.Invalid("the engine returned a plan without its drawing")
	}

	b := model.Blueprint{ID: s.ids.NewID(), TenantID: in.TenantID, ProjectID: in.ProjectID, Name: name, CurrentVersion: 1,
		SourceKind: in.SourceKind, CreatedBy: in.Actor}
	v := planOutcomeVersion(b.ID, model.BlueprintVersionCreate, prompt, in.Actor, out)
	v.Version = 1
	sum := sha256.Sum256(out.DXF)
	orig := model.BlueprintOriginal{BlueprintID: b.ID, Version: 1, Name: drawnFileName(name), MimeType: model.DXFMimeType,
		ByteSize: int64(len(out.DXF)), SHA256: hex.EncodeToString(sum[:]), Data: out.DXF}
	owner := model.OwnerGrant(s.ids.NewID(), in.TenantID, model.ObjectTypeBlueprint, b.ID, in.Actor)
	if err := s.blueprints.CreateImported(ctx, model.CreateImportedBlueprintParams{Blueprint: b, Version: v, Owner: owner, Original: orig}); err != nil {
		return output.BlueprintCreator{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(b), nil, func(ctx context.Context) error {
		return s.blueprints.Delete(ctx, b.ID)
	})
	if err != nil {
		return output.BlueprintCreator{}, err
	}
	if b, err = s.blueprints.Get(ctx, b.ID); err != nil {
		return output.BlueprintCreator{}, err
	}
	v, err = s.blueprints.GetVersion(ctx, b.ID, 1)
	return output.BlueprintCreator{BlueprintDetail: blueprintDetailOutput(blueprintDetail{Blueprint: b, Version: v})}, err
}

// drawnFileName is the name of the DXF the engine drew for a created plan: the blueprint's name, kept short.
func drawnFileName(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) > 100 {
		r = r[:100]
	}
	return string(r) + ".dxf"
}

// validBrief accepts a JSON object; the engine validates its fields.
func validBrief(brief json.RawMessage) error {
	var m map[string]json.RawMessage
	if len(brief) == 0 || json.Unmarshal(brief, &m) != nil || m == nil {
		return model.Invalid("brief must be a JSON object")
	}
	if len(brief) > 64<<10 {
		return model.Invalid("brief must be at most 64 KB")
	}
	return nil
}

// checkSourceKind: a created blueprint comes from a prompt or the form. A file goes through the import.
func checkSourceKind(kind string) error {
	if kind != model.BlueprintSourcePrompt && kind != model.BlueprintSourceForm {
		return model.Invalid("source_kind must be prompt or form (an uploaded file goes through the import)")
	}
	return nil
}
