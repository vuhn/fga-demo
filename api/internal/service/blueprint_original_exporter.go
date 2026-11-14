package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintOriginalExporter struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
}

func NewBlueprintOriginalExporter(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintOriginalExporter {
	return &blueprintOriginalExporter{blueprints: blueprints, engine: engine, authz: authz}
}

// importVersion is the version an import creates; it holds the original file and the plan as first read.
const importVersion = 1

// Export requires can_edit, like the original itself: the file can hold more than the extracted plan. The
// engine writes the differences between the imported plan (version 1) and the chosen version into the original,
// after the Check and before nothing is stored. A blueprint without an original is 404.
func (s *blueprintOriginalExporter) Export(ctx context.Context, actor, tenantID, id string, version int) (entity.PatchedFile, error) {
	b, err := loadBlueprint(ctx, s.blueprints, tenantID, id)
	if err != nil {
		return entity.PatchedFile{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", entity.ObjectTypeBlueprint, id); err != nil {
		return entity.PatchedFile{}, err
	}
	if version == 0 {
		version = b.CurrentVersion
	}
	orig, err := s.blueprints.GetOriginal(ctx, id, importVersion)
	if err != nil {
		return entity.PatchedFile{}, err
	}
	base, err := s.blueprints.GetVersion(ctx, id, importVersion)
	if err != nil {
		return entity.PatchedFile{}, err
	}
	cur, err := s.blueprints.GetVersion(ctx, id, version)
	if err != nil {
		return entity.PatchedFile{}, err
	}
	out, err := s.engine.PatchDXF(ctx, entity.Upload{Name: "original.dxf", MimeType: orig.MimeType, Data: orig.Data}, base.Model, cur.Model)
	if err != nil {
		return entity.PatchedFile{}, err
	}
	out.Name = editedName(orig.Name)
	return out, nil
}

// editedName turns "plan.dxf" into "plan-edited.dxf".
func editedName(name string) string {
	stem := strings.TrimSuffix(name, ".dxf")
	stem = strings.TrimSuffix(stem, ".DXF")
	return stem + "-edited.dxf"
}
