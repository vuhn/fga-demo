package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type blueprintOriginalExporter struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      helper.Authorizer
}

func NewBlueprintOriginalExporter(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz helper.Authorizer) usecase.BlueprintOriginalExporter {
	return &blueprintOriginalExporter{blueprints: blueprints, engine: engine, authz: authz}
}

// Export requires can_edit, like the original itself: the file can hold more than the extracted plan. The
// engine writes the differences between the plan as it was when the file was stored (the import, or the
// conversion) and the chosen version into the original,
// after the Check and before nothing is stored. A blueprint without an original is 404.
func (s *blueprintOriginalExporter) Export(ctx context.Context, in input.BlueprintOriginalExporter) (output.BlueprintOriginalExporter, error) {
	b, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.ID)
	if err != nil {
		return output.BlueprintOriginalExporter{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_edit", model.ObjectTypeBlueprint, in.ID); err != nil {
		return output.BlueprintOriginalExporter{}, err
	}
	if in.Version == 0 {
		in.Version = b.CurrentVersion
	}
	orig, err := s.blueprints.FindOriginal(ctx, in.ID)
	if err != nil {
		return output.BlueprintOriginalExporter{}, err
	}
	if in.Version < orig.Version {
		return output.BlueprintOriginalExporter{}, model.Invalid("version %d is older than the file (version %d); pick version %d or later", in.Version, orig.Version, orig.Version)
	}
	base, err := s.blueprints.GetVersion(ctx, in.ID, orig.Version)
	if err != nil {
		return output.BlueprintOriginalExporter{}, err
	}
	cur, err := s.blueprints.GetVersion(ctx, in.ID, in.Version)
	if err != nil {
		return output.BlueprintOriginalExporter{}, err
	}
	out, err := s.engine.PatchDXF(ctx, model.Upload{Name: "original.dxf", MimeType: orig.MimeType, Data: orig.Data}, base.Model, cur.Model)
	if err != nil {
		return output.BlueprintOriginalExporter{}, err
	}
	out.Name = editedName(orig.Name)
	return output.BlueprintOriginalExporter{PatchedFile: patchedFileOutput(out)}, nil
}

// editedName turns "plan.dxf" into "plan-edited.dxf".
func editedName(name string) string {
	stem := strings.TrimSuffix(name, ".dxf")
	stem = strings.TrimSuffix(stem, ".DXF")
	return stem + "-edited.dxf"
}
