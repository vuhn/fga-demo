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

const projID = "11111111-1111-4111-8111-111111111111"

func (e *fakeEngine) CreatePlan(_ context.Context, brief []byte) (entity.BlueprintOutcome, error) {
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

func (fakeProjects) Get(context.Context, string) (entity.Project, error) {
	return entity.Project{ID: projID, TenantID: "t"}, nil
}

type fakeRelations struct {
	gateway.BlueprintRelationGateway
}

func (fakeRelations) Created(entity.Blueprint) []entity.Tuple { return nil }

type fakeGuard struct{ undone *bool }

func (g fakeGuard) Grant(_ context.Context, _, _ []entity.Tuple, _ func(context.Context) error) error {
	return nil
}
func (g fakeGuard) Revoke(context.Context, []entity.Tuple, []entity.Tuple, func(context.Context) error) error {
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
	created   []entity.Blueprint
	versions  []entity.BlueprintVersion
	originals []entity.BlueprintOriginal
	plain     int
}

func (c *creatingBlueprints) Create(context.Context, entity.Blueprint, entity.BlueprintVersion, entity.Grant) error {
	c.plain++
	return nil
}
func (c *creatingBlueprints) CreateImported(_ context.Context, b entity.Blueprint, v entity.BlueprintVersion, _ entity.Grant, o entity.BlueprintOriginal) error {
	c.created, c.versions, c.originals = append(c.created, b), append(c.versions, v), append(c.originals, o)
	return nil
}

func create(bp *creatingBlueprints, eng *fakeEngine, authz *fakeAuthz, kind string) (usecase.BlueprintDetail, error) {
	return NewBlueprintCreator(fakeProjects{}, bp, fakeRelations{}, eng, authz, fakeGuard{}, fakeIDs{}).Create(context.Background(), "u", "t",
		usecase.CreateBlueprintInput{ProjectID: projID, Name: "House", Brief: json.RawMessage(`{"title":"x"}`), SourceKind: kind, Prompt: "a house"})
}

func TestCreateStoresAPlanLikeAnImport(t *testing.T) {
	bp, eng := &creatingBlueprints{fakeBlueprints: &fakeBlueprints{brief: "null"}}, &fakeEngine{}
	if _, err := create(bp, eng, &fakeAuthz{}, entity.BlueprintSourcePrompt); err != nil {
		t.Fatal(err)
	}
	if len(eng.calls) != 1 || eng.calls[0] != `CreatePlan {"title":"x"}` || bp.plain != 0 {
		t.Fatalf("engine %v plain creates %d", eng.calls, bp.plain)
	}
	v, o := bp.versions[0], bp.originals[0]
	// version 1 has no brief (whatever the engine said), keeps the prompt as its message, and the drawn DXF is the original
	if v.Kind != entity.BlueprintVersionCreate || string(v.Brief) != "null" || v.Message != "a house" || v.Version != 1 {
		t.Fatalf("version %+v", v)
	}
	if string(o.Data) != "DRAWN" || o.Name != "House.dxf" || o.MimeType != entity.DXFMimeType || o.ByteSize != 5 || len(o.SHA256) != 64 || o.Version != 1 {
		t.Fatalf("original %+v", o)
	}
	if bp.created[0].SourceKind != entity.BlueprintSourcePrompt || bp.created[0].Source != nil {
		t.Fatalf("blueprint %+v", bp.created[0])
	}
	if !isImported(entity.BlueprintVersion{Brief: v.Brief}) {
		t.Fatal("a created plan is edited like an imported one")
	}
}

func TestCreateStoresNothingWhenDeniedOrNotBuilt(t *testing.T) {
	bp, eng := &creatingBlueprints{fakeBlueprints: &fakeBlueprints{brief: "null"}}, &fakeEngine{}
	if _, err := create(bp, eng, &fakeAuthz{deny: true}, entity.BlueprintSourcePrompt); !errors.Is(err, entity.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("a denied request reached the engine: %v", eng.calls)
	}
	if _, err := create(bp, &fakeEngine{noModel: true}, &fakeAuthz{}, entity.BlueprintSourceForm); !errors.Is(err, entity.ErrValidation) {
		t.Fatalf("want validation, got %v", err)
	}
	if len(bp.created) != 0 {
		t.Fatalf("stored %v", bp.created)
	}
}

// ---------------------------------------------------------------- converting a plan made from a brief

func (e *fakeEngine) DrawPlan(_ context.Context, model []byte, title string) (entity.BlueprintOutcome, error) {
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
	appended    entity.BlueprintVersion
	original    entity.BlueprintOriginal
	base, n     int
	plainAppend int
}

func (c *convertingBlueprints) AppendVersionWithOriginal(_ context.Context, v entity.BlueprintVersion, base int, o entity.BlueprintOriginal) error {
	c.appended, c.original, c.base, c.n = v, o, base, c.n+1
	return nil
}
func (c *convertingBlueprints) AppendVersion(context.Context, entity.BlueprintVersion, int) error {
	c.plainAppend++
	return nil
}

func TestConvertStoresANewVersionWithoutABriefAndTheDrawingAsItsOriginal(t *testing.T) {
	bp, eng := &convertingBlueprints{fakeBlueprints: &fakeBlueprints{brief: `{"title":"x"}`}}, &fakeEngine{}
	if _, err := NewBlueprintPlanConverter(bp, eng, &fakeAuthz{}).Convert(context.Background(), "u", "t", bpID, 1); err != nil {
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
	if v.Kind != entity.BlueprintVersionEdit || string(v.Brief) != "null" || v.Message != "converted to a DXF plan" || v.CreatedBy != "u" {
		t.Fatalf("version %+v", v)
	}
	if string(o.Data) != "DRAWN" || o.Version != 2 || o.MimeType != entity.DXFMimeType || o.ByteSize != 5 || len(o.SHA256) != 64 || o.BlueprintID != bpID {
		t.Fatalf("original %+v", o)
	}
}

func TestConvertRefusals(t *testing.T) {
	mk := func(brief string) *convertingBlueprints {
		return &convertingBlueprints{fakeBlueprints: &fakeBlueprints{brief: brief}}
	}
	eng := &fakeEngine{}
	call := func(bp *convertingBlueprints, e *fakeEngine, a *fakeAuthz, base int, tenant string) error {
		_, err := NewBlueprintPlanConverter(bp, e, a).Convert(context.Background(), "u", tenant, bpID, base)
		return err
	}
	// denied, stale and another tenant's blueprint never reach the engine
	if err := call(mk(`{"t":1}`), eng, &fakeAuthz{deny: true}, 1, "t"); !errors.Is(err, entity.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if err := call(mk(`{"t":1}`), eng, &fakeAuthz{}, 9, "t"); !errors.Is(err, entity.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	if err := call(mk(`{"t":1}`), eng, &fakeAuthz{}, 1, "other"); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	// a DXF plan is already one
	if err := call(mk("null"), eng, &fakeAuthz{}, 1, "t"); !errors.Is(err, entity.ErrValidation) {
		t.Fatalf("want validation, got %v", err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("engine calls %v", eng.calls)
	}
	// an engine refusal stores nothing
	bp := mk(`{"t":1}`)
	if err := call(bp, &fakeEngine{noModel: true}, &fakeAuthz{}, 1, "t"); !errors.Is(err, entity.ErrValidation) || bp.n != 0 {
		t.Fatalf("err %v writes %d", err, bp.n)
	}
}
