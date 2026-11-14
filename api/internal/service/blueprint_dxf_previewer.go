package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintDXFImportPreviewer struct {
	projects gateway.ProjectGateway
	engine   gateway.BlueprintEngine
	authz    usecase.Authorizer
}

func NewBlueprintDXFImportPreviewer(projects gateway.ProjectGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintDXFImportPreviewer {
	return &blueprintDXFImportPreviewer{projects: projects, engine: engine, authz: authz}
}

// Preview requires can_create_blueprint on the project (the same Check as creating), then has the engine read
// the DXF. Nothing is stored.
func (s *blueprintDXFImportPreviewer) Preview(ctx context.Context, actor, tenantID, projectID string, file entity.Upload, unit string) (entity.BlueprintImport, error) {
	if err := validImportFile(file, unit); err != nil {
		return entity.BlueprintImport{}, err
	}
	if err := requireBlueprintCreate(ctx, s.projects, s.authz, actor, tenantID, projectID); err != nil {
		return entity.BlueprintImport{}, err
	}
	return s.engine.ImportDXF(ctx, file, unit)
}

var importUnits = map[string]bool{"": true, "mm": true, "cm": true, "m": true, "in": true, "ft": true}

func validImportFile(file entity.Upload, unit string) error {
	if len(file.Data) == 0 || len(file.Data) > entity.MaxImportBytes {
		return entity.Invalid("file must be 1 byte to 20 MB")
	}
	if file.MimeType != entity.DXFMimeType {
		return entity.Invalid("only DXF files can be imported")
	}
	if !importUnits[unit] {
		return entity.Invalid("unit must be mm, cm, m, in, or ft")
	}
	return nil
}
