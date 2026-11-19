package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintDrawingRenderer struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
}

func NewBlueprintDrawingRenderer(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintDrawingRenderer {
	return &blueprintDrawingRenderer{blueprints: blueprints, engine: engine, authz: authz}
}

// Render requires can_view (404 when denied) and renders the stored model of
// the current version, or of version when it is > 0.
func (s *blueprintDrawingRenderer) Render(ctx context.Context, actor, tenantID, id, format string, version int) (model.Drawing, error) {
	switch format {
	case model.DrawingSVG, model.DrawingPDF, model.DrawingDXF, model.DrawingIFC:
	default:
		return model.Drawing{}, model.Invalid("format must be svg, pdf, dxf, or ifc")
	}
	b, err := loadBlueprint(ctx, s.blueprints, tenantID, id)
	if err != nil {
		return model.Drawing{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", model.ObjectTypeBlueprint, id); err != nil {
		return model.Drawing{}, err
	}
	if version == 0 {
		version = b.CurrentVersion
	}
	if version < 1 || version > b.CurrentVersion {
		return model.Drawing{}, model.ErrNotFound
	}
	v, err := s.blueprints.GetVersion(ctx, id, version)
	if err != nil {
		return model.Drawing{}, err
	}
	d, err := s.engine.Render(ctx, v.Model, format, model.BlueprintTitleBlock{
		Project: b.Name, Version: v.Version, Issued: v.CreatedAt.Format("2006-01-02"),
	})
	d.Filename = fmt.Sprintf("%s-v%d.%s", fileStem(b.Name), v.Version, format)
	return d, err
}

// fileStem keeps letters, digits, '-' and '_' of a name for a download filename.
func fileStem(name string) string {
	stem := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r == ' ':
			return '-'
		}
		return -1
	}, name)
	if stem == "" {
		return "blueprint"
	}
	return stem
}
