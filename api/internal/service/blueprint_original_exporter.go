package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
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

// Export requires can_edit, like the original itself: the file can hold more than the extracted plan. The
// engine writes the differences between the plan as it was when the file was stored (the import, or the
// conversion) and the chosen version into the original,
// after the Check and before nothing is stored. A blueprint without an original is 404.
func (s *blueprintOriginalExporter) Export(ctx context.Context, actor, tenantID, id string, version int) (model.PatchedFile, error) {
	b, err := loadBlueprint(ctx, s.blueprints, tenantID, id)
	if err != nil {
		return model.PatchedFile{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", model.ObjectTypeBlueprint, id); err != nil {
		return model.PatchedFile{}, err
	}
	if version == 0 {
		version = b.CurrentVersion
	}
	orig, err := s.blueprints.FindOriginal(ctx, id)
	if err != nil {
		return model.PatchedFile{}, err
	}
	if version < orig.Version {
		return model.PatchedFile{}, model.Invalid("version %d is older than the file (version %d); pick version %d or later", version, orig.Version, orig.Version)
	}
	base, err := s.blueprints.GetVersion(ctx, id, orig.Version)
	if err != nil {
		return model.PatchedFile{}, err
	}
	cur, err := s.blueprints.GetVersion(ctx, id, version)
	if err != nil {
		return model.PatchedFile{}, err
	}
	out, err := s.engine.PatchDXF(ctx, model.Upload{Name: "original.dxf", MimeType: orig.MimeType, Data: orig.Data}, base.Model, cur.Model)
	if err != nil {
		return model.PatchedFile{}, err
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
