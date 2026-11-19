package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

func TestRequireBriefRefusesImportedPlans(t *testing.T) {
	for _, brief := range []string{"null", "", " null "} {
		err := requireBrief(model.BlueprintVersion{Brief: json.RawMessage(brief)})
		if !errors.Is(err, model.ErrValidation) {
			t.Errorf("brief %q: got %v, want validation error", brief, err)
		}
	}
	if err := requireBrief(model.BlueprintVersion{Brief: json.RawMessage(`{"title":"x"}`)}); err != nil {
		t.Errorf("a real brief must pass: %v", err)
	}
}

func TestValidImportFileAcceptsPicturesAndPDF(t *testing.T) {
	for _, mt := range []string{model.PNGMimeType, model.JPEGMimeType, model.PDFMimeType} {
		f := model.Upload{Name: "plan", MimeType: mt, Data: []byte("x")}
		if err := validImportFile(f, "", model.ConvertOptions{Scale: 100, LongestM: 12}); err != nil {
			t.Errorf("%s: %v", mt, err)
		}
	}
}

func TestValidImportFile(t *testing.T) {
	ok := model.Upload{Name: "a.dxf", MimeType: model.DXFMimeType, Data: []byte("x")}
	if err := validImportFile(ok, "mm", model.ConvertOptions{}); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		f    model.Upload
		unit string
		opts model.ConvertOptions
	}{
		"empty":      {model.Upload{MimeType: model.DXFMimeType}, "", model.ConvertOptions{}},
		"not a plan": {model.Upload{Data: []byte("x"), MimeType: "image/gif"}, "", model.ConvertOptions{}},
		"bad scale":  {model.Upload{Data: []byte("x"), MimeType: model.PNGMimeType}, "", model.ConvertOptions{Scale: 9000}},
		"bad length": {model.Upload{Data: []byte("x"), MimeType: model.PDFMimeType}, "", model.ConvertOptions{LongestM: 0.2}},
		"bad unit":   {ok, "parsec", model.ConvertOptions{}},
		"too large":  {model.Upload{MimeType: model.DXFMimeType, Data: make([]byte, model.MaxImportBytes+1)}, "", model.ConvertOptions{}},
	}
	for name, c := range cases {
		if err := validImportFile(c.f, c.unit, c.opts); !errors.Is(err, model.ErrValidation) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

type convertEngine struct {
	gateway.BlueprintEngine
	called bool
}

func (e *convertEngine) ConvertToDXF(_ context.Context, f model.Upload, o model.ConvertOptions) (model.BlueprintConversion, error) {
	e.called = true
	return model.BlueprintConversion{DXF: []byte("dxf of " + f.Name), Report: json.RawMessage(`{"method":"raster-trace"}`)}, nil
}

func TestImportableDXF(t *testing.T) {
	eng := &convertEngine{}
	dxf := model.Upload{Name: "a.dxf", MimeType: model.DXFMimeType, Data: []byte("d")}
	got, conv, err := importableDXF(context.Background(), eng, dxf, model.ConvertOptions{})
	if err != nil || eng.called || conv != nil || got.Name != "a.dxf" {
		t.Fatalf("a DXF is used as is: %v %v %v %v", got.Name, conv, eng.called, err)
	}
	got, conv, err = importableDXF(context.Background(), eng, model.Upload{Name: "house.plan.png", MimeType: model.PNGMimeType, Data: []byte("p")}, model.ConvertOptions{LongestM: 12})
	if err != nil || !eng.called || got.Name != "house.plan.dxf" || got.MimeType != model.DXFMimeType || string(got.Data) != "dxf of house.plan.png" || conv == nil {
		t.Fatalf("a picture is converted: %+v %s %v", got, conv, err)
	}
}

func TestImportDeniedNeverReachesTheEngineOrStorage(t *testing.T) {
	bp, eng := &creatingBlueprints{fakeBlueprints: &fakeBlueprints{brief: "null"}}, &convertEngine{}
	imp := NewBlueprintImporter(fakeProjects{}, bp, fakeRelations{}, eng, &fakeAuthz{deny: true}, fakeGuard{}, fakeIDs{})
	_, err := imp.Import(context.Background(), "u", "t", usecase.ImportBlueprintInput{ProjectID: projID, Name: "House", PlanIndex: 1,
		File: model.Upload{Name: "p.png", MimeType: model.PNGMimeType, Data: []byte("x")}})
	if !errors.Is(err, model.ErrForbidden) || eng.called || len(bp.created) != 0 {
		t.Fatalf("err %v, engine called %v, stored %d", err, eng.called, len(bp.created))
	}
}
