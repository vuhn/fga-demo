package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

const projID = "11111111-1111-4111-8111-111111111111"

func (e *fakeEngine) CreatePlan(_ context.Context, brief []byte) (model.BlueprintOutcome, error) {
	e.calls = append(e.calls, "CreatePlan "+string(brief))
	o, err := e.out()
	o.Brief = nil // the engine returns no brief for a created plan
	if e.noModel {
		o.Model = nil
		o.Issues = []string{"too big"}
	} else {
		o.DXF = []byte("DRAWN")
	}
	return o, err
}

type fakeProjects struct{ gateway.ProjectGateway }

func (fakeProjects) Get(context.Context, string) (model.Project, error) {
	return model.Project{ID: projID, TenantID: "t"}, nil
}

type fakeRelations struct {
	gateway.BlueprintRelationGateway
}

func (fakeRelations) Created(model.Blueprint) []model.Tuple { return nil }

type fakeGuard struct{ undone *bool }

func (g fakeGuard) Grant(_ context.Context, _, _ []model.Tuple, _ func(context.Context) error) error {
	return nil
}
func (g fakeGuard) Revoke(context.Context, []model.Tuple, []model.Tuple, func(context.Context) error) error {
	return nil
}
func (g fakeGuard) RevokeObject(context.Context, string, string, func(context.Context) error) error {
	return nil
}

type fakeIDs struct{}

func (fakeIDs) NewID() string { return "22222222-2222-4222-8222-222222222222" }

// creating blueprints records what was stored
type creatingBlueprints struct {
	*fakeBlueprints
	created   []model.Blueprint
	versions  []model.BlueprintVersion
	originals []model.BlueprintOriginal
	plain     int
}

func (c *creatingBlueprints) Create(context.Context, model.Blueprint, model.BlueprintVersion, model.Grant) error {
	c.plain++
	return nil
}
func (c *creatingBlueprints) CreateImported(_ context.Context, p model.CreateImportedBlueprintParams) error {
	b, v, o := p.Blueprint, p.Version, p.Original
	c.created, c.versions, c.originals = append(c.created, b), append(c.versions, v), append(c.originals, o)
	return nil
}

func create(bp *creatingBlueprints, eng *fakeEngine, authz *fakeAuthz, kind string) (output.BlueprintCreator, error) {
	return NewBlueprintCreator(fakeProjects{}, bp, fakeRelations{}, eng, authz, fakeGuard{}, fakeIDs{}).Create(context.Background(),
		input.BlueprintCreator{Actor: "u", TenantID: "t", ProjectID: projID, Name: "House", Brief: json.RawMessage(`{"title":"x"}`), SourceKind: kind, Prompt: "a house"})
}

func TestCreateStoresAPlanLikeAnImport(t *testing.T) {
	bp, eng := &creatingBlueprints{fakeBlueprints: &fakeBlueprints{brief: "null"}}, &fakeEngine{}
	if _, err := create(bp, eng, &fakeAuthz{}, model.BlueprintSourcePrompt); err != nil {
		t.Fatal(err)
	}
	if len(eng.calls) != 1 || eng.calls[0] != `CreatePlan {"title":"x"}` || bp.plain != 0 {
		t.Fatalf("engine %v plain creates %d", eng.calls, bp.plain)
	}
	v, o := bp.versions[0], bp.originals[0]
	// version 1 has no brief (whatever the engine said), keeps the prompt as its message, and the drawn DXF is the original
	if v.Kind != model.BlueprintVersionCreate || string(v.Brief) != "null" || v.Message != "a house" || v.Version != 1 {
		t.Fatalf("version %+v", v)
	}
	if string(o.Data) != "DRAWN" || o.Name != "House.dxf" || o.MimeType != model.DXFMimeType || o.ByteSize != 5 || len(o.SHA256) != 64 || o.Version != 1 {
		t.Fatalf("original %+v", o)
	}
	if bp.created[0].SourceKind != model.BlueprintSourcePrompt || bp.created[0].Source != nil {
		t.Fatalf("blueprint %+v", bp.created[0])
	}
	if !isImported(model.BlueprintVersion{Brief: v.Brief}) {
		t.Fatal("a created plan is edited like an imported one")
	}
}

func TestCreateStoresNothingWhenDeniedOrNotBuilt(t *testing.T) {
	bp, eng := &creatingBlueprints{fakeBlueprints: &fakeBlueprints{brief: "null"}}, &fakeEngine{}
	if _, err := create(bp, eng, &fakeAuthz{deny: true}, model.BlueprintSourcePrompt); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("a denied request reached the engine: %v", eng.calls)
	}
	if _, err := create(bp, &fakeEngine{noModel: true}, &fakeAuthz{}, model.BlueprintSourceForm); !errors.Is(err, model.ErrValidation) {
		t.Fatalf("want validation, got %v", err)
	}
	if len(bp.created) != 0 {
		t.Fatalf("stored %v", bp.created)
	}
}

// ---------------------------------------------------------------- converting a plan made from a brief

func (e *fakeEngine) DrawPlan(_ context.Context, model []byte, title string) (model.BlueprintOutcome, error) {
	e.calls = append(e.calls, "DrawPlan "+string(model)+" "+title)
	o, err := e.out()
	o.Brief = nil
	if e.noModel {
		o.Model = nil
		o.Issues = []string{"cannot convert"}
	} else {
		o.DXF = []byte("DRAWN")
	}
	return o, err
}

type convertingBlueprints struct {
	*fakeBlueprints
	appended    model.BlueprintVersion
	original    model.BlueprintOriginal
	base, n     int
	plainAppend int
}

func (c *convertingBlueprints) AppendVersionWithOriginal(_ context.Context, v model.BlueprintVersion, base int, o model.BlueprintOriginal) (model.Blueprint, model.BlueprintVersion, error) {
	c.appended, c.original, c.base, c.n = v, o, base, c.n+1
	v.Version = base + 1
	return model.Blueprint{ID: bpID, TenantID: "t", CurrentVersion: base + 1}, v, nil
}
func (c *convertingBlueprints) AppendVersion(context.Context, model.BlueprintVersion, int) (model.Blueprint, model.BlueprintVersion, error) {
	c.plainAppend++
	return model.Blueprint{}, model.BlueprintVersion{}, nil
}

func TestConvertStoresANewVersionWithoutABriefAndTheDrawingAsItsOriginal(t *testing.T) {
	bp, eng := &convertingBlueprints{fakeBlueprints: &fakeBlueprints{brief: `{"title":"x"}`}}, &fakeEngine{}
	if _, err := NewBlueprintPlanConverter(bp, eng, &fakeAuthz{}).Convert(context.Background(), input.BlueprintPlanConverter{Actor: "u", TenantID: "t", ID: bpID, BaseVersion: 1}); err != nil {
		t.Fatal(err)
	}
	// the engine gets the stored model and the blueprint name, no ids and no brief
	if len(eng.calls) != 1 || eng.calls[0] != `DrawPlan {"a":1} ` {
		t.Fatalf("engine calls %q", eng.calls)
	}
	if bp.n != 1 || bp.plainAppend != 0 || bp.base != 1 {
		t.Fatalf("writes %+v", bp)
	}
	v, o := bp.appended, bp.original
	if v.Kind != model.BlueprintVersionEdit || string(v.Brief) != "null" || v.Message != "converted to a DXF plan" || v.CreatedBy != "u" {
		t.Fatalf("version %+v", v)
	}
	if string(o.Data) != "DRAWN" || o.Version != 2 || o.MimeType != model.DXFMimeType || o.ByteSize != 5 || len(o.SHA256) != 64 || o.BlueprintID != bpID {
		t.Fatalf("original %+v", o)
	}
}

func TestConvertRefusals(t *testing.T) {
	mk := func(brief string) *convertingBlueprints {
		return &convertingBlueprints{fakeBlueprints: &fakeBlueprints{brief: brief}}
	}
	eng := &fakeEngine{}
	call := func(bp *convertingBlueprints, e *fakeEngine, a *fakeAuthz, base int, tenant string) error {
		_, err := NewBlueprintPlanConverter(bp, e, a).Convert(context.Background(), input.BlueprintPlanConverter{Actor: "u", TenantID: tenant, ID: bpID, BaseVersion: base})
		return err
	}
	// denied, stale and another tenant's blueprint never reach the engine
	if err := call(mk(`{"t":1}`), eng, &fakeAuthz{deny: true}, 1, "t"); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if err := call(mk(`{"t":1}`), eng, &fakeAuthz{}, 9, "t"); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	if err := call(mk(`{"t":1}`), eng, &fakeAuthz{}, 1, "other"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	// a DXF plan is already one
	if err := call(mk("null"), eng, &fakeAuthz{}, 1, "t"); !errors.Is(err, model.ErrValidation) {
		t.Fatalf("want validation, got %v", err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("engine calls %v", eng.calls)
	}
	// an engine refusal stores nothing
	bp := mk(`{"t":1}`)
	if err := call(bp, &fakeEngine{noModel: true}, &fakeAuthz{}, 1, "t"); !errors.Is(err, model.ErrValidation) || bp.n != 0 {
		t.Fatalf("err %v writes %d", err, bp.n)
	}
}
