package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type blueprintCreator struct {
	projects   gateway.ProjectGateway
	blueprints gateway.BlueprintGateway
	relations  gateway.BlueprintRelationGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
	guard      helper.WriteGuard
	ids        gateway.IDGenerator
}

func NewBlueprintCreator(projects gateway.ProjectGateway, blueprints gateway.BlueprintGateway, relations gateway.BlueprintRelationGateway,
	engine gateway.BlueprintEngine, authz usecase.Authorizer, guard helper.WriteGuard, ids gateway.IDGenerator) usecase.BlueprintCreator {
	return &blueprintCreator{projects: projects, blueprints: blueprints, relations: relations, engine: engine, authz: authz, guard: guard, ids: ids}
}

// Create requires can_create_blueprint on the project. The engine builds the
// brief (shrinking rooms when it is too big) before anything is written; a
// brief that still cannot be built is 400. Then Postgres, then OpenFGA.
func (s *blueprintCreator) Create(ctx context.Context, actor, tenantID string, in usecase.CreateBlueprintInput) (usecase.BlueprintDetail, error) {
	name, err := entity.CleanName("name", in.Name)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := validBrief(in.Brief); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	source, err := cleanSource(in.SourceKind, in.Source)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	prompt := strings.TrimSpace(in.Prompt)
	if len([]rune(prompt)) > 2000 {
		return usecase.BlueprintDetail{}, entity.Invalid("prompt must be at most 2000 characters")
	}
	if err := requireBlueprintCreate(ctx, s.projects, s.authz, actor, tenantID, in.ProjectID); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	out, err := s.engine.Generate(ctx, in.Brief, nil, true)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}

	b := entity.Blueprint{ID: s.ids.NewID(), TenantID: tenantID, ProjectID: in.ProjectID, Name: name, CurrentVersion: 1,
		SourceKind: in.SourceKind, Source: source, CreatedBy: actor}
	v := outcomeVersion(b.ID, entity.BlueprintVersionCreate, prompt, actor, out)
	v.Version = 1
	owner := entity.OwnerGrant(s.ids.NewID(), tenantID, entity.ObjectTypeBlueprint, b.ID, actor)
	if err := s.blueprints.Create(ctx, b, v, owner); err != nil {
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

// validBrief accepts a JSON object; the engine validates its fields.
func validBrief(brief json.RawMessage) error {
	var m map[string]json.RawMessage
	if len(brief) == 0 || json.Unmarshal(brief, &m) != nil || m == nil {
		return entity.Invalid("brief must be a JSON object")
	}
	if len(brief) > 64<<10 {
		return entity.Invalid("brief must be at most 64 KB")
	}
	return nil
}

func cleanSource(kind string, src *entity.BlueprintSource) (*entity.BlueprintSource, error) {
	switch kind {
	case entity.BlueprintSourcePrompt, entity.BlueprintSourceForm:
		return nil, nil
	case entity.BlueprintSourceSketch:
	default:
		return nil, entity.Invalid("source_kind must be prompt, sketch, or form")
	}
	if src == nil {
		return nil, nil
	}
	name, err := entity.CleanName("source.name", src.Name)
	if err != nil {
		return nil, err
	}
	if !sketchTypes[strings.TrimSpace(src.MimeType)] {
		return nil, entity.Invalid("source.mime_type must be image/png, image/jpeg, or application/pdf")
	}
	if src.ByteSize < 0 || src.ByteSize > entity.MaxSketchBytes {
		return nil, entity.Invalid("source.byte_size must be 0 to 10 MB")
	}
	return &entity.BlueprintSource{Name: name, MimeType: strings.TrimSpace(src.MimeType), ByteSize: src.ByteSize}, nil
}
