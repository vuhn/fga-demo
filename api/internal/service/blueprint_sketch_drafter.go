package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// sketchTypes are the uploads the engine reads (PDF: first page only; DXF: text labels and outlines, no vision model).
var sketchTypes = map[string]bool{"image/png": true, "image/jpeg": true, "application/pdf": true, entity.DXFMimeType: true}

type blueprintSketchDrafter struct {
	projects gateway.ProjectGateway
	engine   gateway.BlueprintEngine
	authz    usecase.Authorizer
}

func NewBlueprintSketchDrafter(projects gateway.ProjectGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintSketchDrafter {
	return &blueprintSketchDrafter{projects: projects, engine: engine, authz: authz}
}

// DraftFromSketch requires can_create_blueprint on the project, then asks the
// engine to read the sketch into a brief. The file is not stored.
func (s *blueprintSketchDrafter) DraftFromSketch(ctx context.Context, actor, tenantID, projectID string, file entity.Upload, note string) (entity.BlueprintDraft, error) {
	if !sketchTypes[file.MimeType] {
		return entity.BlueprintDraft{}, entity.Invalid("file must be PNG, JPG, PDF, or DXF")
	}
	if len(file.Data) == 0 || len(file.Data) > entity.MaxSketchBytes {
		return entity.BlueprintDraft{}, entity.Invalid("file must be 1 byte to 10 MB")
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > 1000 {
		return entity.BlueprintDraft{}, entity.Invalid("note length must be at most 1000")
	}
	if err := requireBlueprintCreate(ctx, s.projects, s.authz, actor, tenantID, projectID); err != nil {
		return entity.BlueprintDraft{}, err
	}
	return s.engine.BriefFromSketch(ctx, file, note)
}
