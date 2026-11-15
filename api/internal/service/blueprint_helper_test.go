package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

func TestRequireBriefRefusesImportedPlans(t *testing.T) {
	for _, brief := range []string{"null", "", " null "} {
		err := requireBrief(entity.BlueprintVersion{Brief: json.RawMessage(brief)})
		if !errors.Is(err, entity.ErrValidation) {
			t.Errorf("brief %q: got %v, want validation error", brief, err)
		}
	}
	if err := requireBrief(entity.BlueprintVersion{Brief: json.RawMessage(`{"title":"x"}`)}); err != nil {
		t.Errorf("a real brief must pass: %v", err)
	}
}

func TestValidImportFileAcceptsPicturesAndPDF(t *testing.T) {
	for _, mt := range []string{entity.PNGMimeType, entity.JPEGMimeType, entity.PDFMimeType} {
		f := entity.Upload{Name: "plan", MimeType: mt, Data: []byte("x")}
		if err := validImportFile(f, "", entity.ConvertOptions{Scale: 100, LongestM: 12}); err != nil {
			t.Errorf("%s: %v", mt, err)
		}
	}
}

func TestValidImportFile(t *testing.T) {
	ok := entity.Upload{Name: "a.dxf", MimeType: entity.DXFMimeType, Data: []byte("x")}
	if err := validImportFile(ok, "mm", entity.ConvertOptions{}); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		f    entity.Upload
		unit string
		opts entity.ConvertOptions
	}{
		"empty":      {entity.Upload{MimeType: entity.DXFMimeType}, "", entity.ConvertOptions{}},
		"not a plan": {entity.Upload{Data: []byte("x"), MimeType: "image/gif"}, "", entity.ConvertOptions{}},
		"bad scale":  {entity.Upload{Data: []byte("x"), MimeType: entity.PNGMimeType}, "", entity.ConvertOptions{Scale: 9000}},
		"bad length": {entity.Upload{Data: []byte("x"), MimeType: entity.PDFMimeType}, "", entity.ConvertOptions{LongestM: 0.2}},
		"bad unit":   {ok, "parsec", entity.ConvertOptions{}},
		"too large":  {entity.Upload{MimeType: entity.DXFMimeType, Data: make([]byte, entity.MaxImportBytes+1)}, "", entity.ConvertOptions{}},
	}
	for name, c := range cases {
		if err := validImportFile(c.f, c.unit, c.opts); !errors.Is(err, entity.ErrValidation) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

type convertEngine struct {
	gateway.BlueprintEngine
	called bool
}

func (e *convertEngine) ConvertToDXF(_ context.Context, f entity.Upload, o entity.ConvertOptions) (entity.BlueprintConversion, error) {
	e.called = true
	return entity.BlueprintConversion{DXF: []byte("dxf of " + f.Name), Report: json.RawMessage(`{"method":"raster-trace"}`)}, nil
}

func TestImportableDXF(t *testing.T) {
	eng := &convertEngine{}
	dxf := entity.Upload{Name: "a.dxf", MimeType: entity.DXFMimeType, Data: []byte("d")}
	got, conv, err := importableDXF(context.Background(), eng, dxf, entity.ConvertOptions{})
	if err != nil || eng.called || conv != nil || got.Name != "a.dxf" {
		t.Fatalf("a DXF is used as is: %v %v %v %v", got.Name, conv, eng.called, err)
	}
	got, conv, err = importableDXF(context.Background(), eng, entity.Upload{Name: "house.plan.png", MimeType: entity.PNGMimeType, Data: []byte("p")}, entity.ConvertOptions{LongestM: 12})
	if err != nil || !eng.called || got.Name != "house.plan.dxf" || got.MimeType != entity.DXFMimeType || string(got.Data) != "dxf of house.plan.png" || conv == nil {
		t.Fatalf("a picture is converted: %+v %s %v", got, conv, err)
	}
}

func TestImportDeniedNeverReachesTheEngineOrStorage(t *testing.T) {
	bp, eng := &creatingBlueprints{fakeBlueprints: &fakeBlueprints{brief: "null"}}, &convertEngine{}
	imp := NewBlueprintImporter(fakeProjects{}, bp, fakeRelations{}, eng, &fakeAuthz{deny: true}, fakeGuard{}, fakeIDs{})
	_, err := imp.Import(context.Background(), "u", "t", usecase.ImportBlueprintInput{ProjectID: projID, Name: "House", PlanIndex: 1,
		File: entity.Upload{Name: "p.png", MimeType: entity.PNGMimeType, Data: []byte("x")}})
	if !errors.Is(err, entity.ErrForbidden) || eng.called || len(bp.created) != 0 {
		t.Fatalf("err %v, engine called %v, stored %d", err, eng.called, len(bp.created))
	}
}
