package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

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
// the file: a DXF as drawn, a PDF or image converted to a DXF first. Nothing is stored.
func (s *blueprintDXFImportPreviewer) Preview(ctx context.Context, actor, tenantID, projectID string, file entity.Upload, unit string, opts entity.ConvertOptions) (entity.BlueprintImport, error) {
	if err := validImportFile(file, unit, opts); err != nil {
		return entity.BlueprintImport{}, err
	}
	if err := requireBlueprintCreate(ctx, s.projects, s.authz, actor, tenantID, projectID); err != nil {
		return entity.BlueprintImport{}, err
	}
	dxf, conv, err := importableDXF(ctx, s.engine, file, opts)
	if err != nil {
		return entity.BlueprintImport{}, err
	}
	if conv != nil {
		unit = "" // the converted DXF is in metres
	}
	res, err := s.engine.ImportDXF(ctx, dxf, unit)
	res.Conversion = conv
	return res, err
}

var importUnits = map[string]bool{"": true, "mm": true, "cm": true, "m": true, "in": true, "ft": true}

var convertibleTypes = map[string]bool{entity.PNGMimeType: true, entity.JPEGMimeType: true, entity.PDFMimeType: true}

func validImportFile(file entity.Upload, unit string, opts entity.ConvertOptions) error {
	if len(file.Data) == 0 || len(file.Data) > entity.MaxImportBytes {
		return entity.Invalid("file must be 1 byte to 20 MB")
	}
	if file.MimeType != entity.DXFMimeType && !convertibleTypes[file.MimeType] {
		return entity.Invalid("file must be a DXF, PDF, PNG, or JPG")
	}
	if !importUnits[unit] {
		return entity.Invalid("unit must be mm, cm, m, in, or ft")
	}
	if opts.Scale < 0 || opts.Scale > 5000 {
		return entity.Invalid("scale must be 1:1 to 1:5000")
	}
	if opts.LongestM != 0 && (opts.LongestM < 1 || opts.LongestM > 500) {
		return entity.Invalid("the drawing's longest side must be 1 to 500 m")
	}
	return nil
}

// importableDXF is the DXF the engine reads: the upload itself, or a PDF/image converted to one (named after the
// upload, in metres). The second value is the conversion report, nil for a DXF.
func importableDXF(ctx context.Context, engine gateway.BlueprintEngine, file entity.Upload, opts entity.ConvertOptions) (entity.Upload, json.RawMessage, error) {
	if file.MimeType == entity.DXFMimeType {
		return file, nil, nil
	}
	conv, err := engine.ConvertToDXF(ctx, file, opts)
	if err != nil {
		return entity.Upload{}, nil, err
	}
	stem := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))
	return entity.Upload{Name: stem + ".dxf", MimeType: entity.DXFMimeType, Data: conv.DXF}, conv.Report, nil
}
