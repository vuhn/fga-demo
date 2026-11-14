package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type blueprintImporter struct {
	projects   gateway.ProjectGateway
	blueprints gateway.BlueprintGateway
	relations  gateway.BlueprintRelationGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
	guard      helper.WriteGuard
	ids        gateway.IDGenerator
}

func NewBlueprintImporter(projects gateway.ProjectGateway, blueprints gateway.BlueprintGateway, relations gateway.BlueprintRelationGateway,
	engine gateway.BlueprintEngine, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.BlueprintImporter {
	return &blueprintImporter{projects: projects, blueprints: blueprints, relations: relations, engine: engine, authz: authz, guard: guard, ids: ids}
}

// Import requires can_create_blueprint on the project. The engine reads the file (before any write), the chosen
// plan becomes version 1 (kind import, no brief) and the original is stored with it in the same transaction.
// Then OpenFGA; a failure there deletes the Postgres rows, like every create.
func (s *blueprintImporter) Import(ctx context.Context, actor, tenantID string, in usecase.ImportBlueprintInput) (usecase.BlueprintDetail, error) {
	name, err := entity.CleanName("name", in.Name)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := validImportFile(in.File, in.Unit); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if in.PlanIndex < 1 {
		return usecase.BlueprintDetail{}, entity.Invalid("plan must be 1 or more")
	}
	fileName, err := entity.CleanName("file name", in.File.Name)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBlueprintCreate(ctx, s.projects, s.authz, actor, tenantID, in.ProjectID); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	res, err := s.engine.ImportDXF(ctx, in.File, in.Unit)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	var plan *entity.BlueprintImportPlan
	for i := range res.Plans {
		if res.Plans[i].Index == in.PlanIndex {
			plan = &res.Plans[i]
		}
	}
	if plan == nil {
		return usecase.BlueprintDetail{}, entity.Invalid("the file has %d plan(s); plan %d does not exist", len(res.Plans), in.PlanIndex)
	}

	sum := sha256.Sum256(in.File.Data)
	size := int64(len(in.File.Data))
	b := entity.Blueprint{ID: s.ids.NewID(), TenantID: tenantID, ProjectID: in.ProjectID, Name: name, CurrentVersion: 1,
		SourceKind: entity.BlueprintSourceImport, CreatedBy: actor,
		Source: &entity.BlueprintSource{Name: fileName, MimeType: entity.DXFMimeType, ByteSize: size}}
	v := entity.BlueprintVersion{BlueprintID: b.ID, Version: 1, Kind: entity.BlueprintVersionImport, Message: "imported " + fileName,
		Brief: json.RawMessage("null"), Model: plan.Model, Checks: plan.Checks, Notes: importNotes(res, *plan), OK: plan.OK, CreatedBy: actor}
	orig := entity.BlueprintOriginal{BlueprintID: b.ID, Version: 1, Name: fileName, MimeType: entity.DXFMimeType, ByteSize: size,
		SHA256: hex.EncodeToString(sum[:]), Data: in.File.Data}
	owner := entity.OwnerGrant(s.ids.NewID(), tenantID, entity.ObjectTypeBlueprint, b.ID, actor)
	if err := s.blueprints.CreateImported(ctx, b, v, owner, orig); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	err = s.guard.Grant(ctx, s.relations.Created(b), nil, func(ctx context.Context) error {
		return s.blueprints.Delete(ctx, b.ID)
	})
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if b, err = s.blueprints.Get(ctx, b.ID); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	v, err = s.blueprints.GetVersion(ctx, b.ID, 1)
	return usecase.BlueprintDetail{Blueprint: b, Version: v}, err
}

// importNotes are the lines shown under the version: which plan, the unit, and what the engine warned about.
func importNotes(res entity.BlueprintImport, p entity.BlueprintImportPlan) []string {
	var rep struct {
		Units      string   `json:"units"`
		UnitSource string   `json:"unit_source"`
		Warnings   []string `json:"warnings"`
		Unmapped   []string `json:"unmapped_texts"`
	}
	_ = json.Unmarshal(res.Report, &rep)
	notes := []string{fmt.Sprintf("imported plan %d of %d from the DXF: %d rooms, %.1f m²", p.Index, len(res.Plans), p.RoomCount, p.NetArea)}
	if rep.Units != "" {
		notes = append(notes, fmt.Sprintf("units: %s (%s)", rep.Units, rep.UnitSource))
	}
	notes = append(notes, rep.Warnings...)
	for _, u := range rep.Unmapped {
		notes = append(notes, "label not recognised: "+u)
	}
	return notes
}
