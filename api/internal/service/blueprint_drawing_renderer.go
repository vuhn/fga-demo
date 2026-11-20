package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type blueprintDrawingRenderer struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      helper.Authorizer
}

func NewBlueprintDrawingRenderer(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz helper.Authorizer) usecase.BlueprintDrawingRenderer {
	return &blueprintDrawingRenderer{blueprints: blueprints, engine: engine, authz: authz}
}

// Render requires can_view (404 when denied) and renders the stored model of
// the current version, or of version when it is > 0.
func (s *blueprintDrawingRenderer) Render(ctx context.Context, in input.BlueprintDrawingRenderer) (output.BlueprintDrawingRenderer, error) {
	switch in.Format {
	case model.DrawingSVG, model.DrawingPDF, model.DrawingDXF, model.DrawingIFC:
	default:
		return output.BlueprintDrawingRenderer{}, model.Invalid("format must be svg, pdf, dxf, or ifc")
	}
	b, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.ID)
	if err != nil {
		return output.BlueprintDrawingRenderer{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeBlueprint, in.ID); err != nil {
		return output.BlueprintDrawingRenderer{}, err
	}
	if in.Version == 0 {
		in.Version = b.CurrentVersion
	}
	if in.Version < 1 || in.Version > b.CurrentVersion {
		return output.BlueprintDrawingRenderer{}, model.ErrNotFound
	}
	v, err := s.blueprints.GetVersion(ctx, in.ID, in.Version)
	if err != nil {
		return output.BlueprintDrawingRenderer{}, err
	}
	d, err := s.engine.Render(ctx, v.Model, in.Format, model.BlueprintTitleBlock{
		Project: b.Name, Version: v.Version, Issued: v.CreatedAt.Format("2006-01-02"),
	})
	d.Filename = fmt.Sprintf("%s-v%d.%s", fileStem(b.Name), v.Version, in.Format)
	return output.BlueprintDrawingRenderer{Drawing: drawingOutput(d)}, err
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
