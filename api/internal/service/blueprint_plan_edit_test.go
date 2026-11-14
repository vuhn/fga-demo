package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

func (e *fakeEngine) EditPlan(_ context.Context, _, ops []byte) (entity.BlueprintOutcome, error) {
	e.calls = append(e.calls, "EditPlan "+string(ops))
	o, err := e.out()
	o.Notes = []string{"edited: rename room"}
	return o, err
}

func (e *fakeEngine) PatchDXF(_ context.Context, orig entity.Upload, base, cur []byte) (entity.PatchedFile, error) {
	e.calls = append(e.calls, "PatchDXF "+orig.Name+" "+string(orig.Data)+" "+string(base)+" "+string(cur))
	return entity.PatchedFile{MimeType: orig.MimeType, Data: []byte("PATCHED"), Report: json.RawMessage(`{"applied":[]}`)}, e.err
}

// withOriginal is a fakeBlueprints that also holds the original file of version 1.
type withOriginal struct {
	*fakeBlueprints
	none    bool
	version int // the version that holds the file (default 1)
}

func (w withOriginal) GetOriginal(context.Context, string, int) (entity.BlueprintOriginal, error) {
	if w.none {
		return entity.BlueprintOriginal{}, entity.ErrNotFound
	}
	return entity.BlueprintOriginal{Name: "plan.dxf", MimeType: entity.DXFMimeType, Data: []byte("ORIGINAL")}, nil
}

func (w withOriginal) FindOriginal(ctx context.Context, id string) (entity.BlueprintOriginal, error) {
	o, err := w.GetOriginal(ctx, id, 1)
	o.Version = w.version
	if o.Version == 0 {
		o.Version = 1
	}
	return o, err
}

func TestPlanEditStoresAnEditVersionWithANullBrief(t *testing.T) {
	bp, eng := &fakeBlueprints{brief: "null"}, &fakeEngine{}
	ops := json.RawMessage(`{"ops":[{"op":"rename_room","room":"Bath","name":"Den"}]}`)
	if _, err := NewBlueprintPlanEditor(bp, eng, &fakeAuthz{}).Edit(context.Background(), "u", "t", bpID, ops, 1); err != nil {
		t.Fatal(err)
	}
	if len(eng.calls) != 1 || eng.calls[0] != "EditPlan "+string(ops) {
		t.Fatalf("engine calls %v", eng.calls)
	}
	v := bp.appended[0]
	if v.Kind != entity.BlueprintVersionEdit || string(v.Brief) != "null" || v.Message != "edited: rename room" || v.CreatedBy != "u" {
		t.Fatalf("stored %+v", v)
	}
}

func TestPlanEditRefusals(t *testing.T) {
	ops := json.RawMessage(`{"ops":[]}`)
	// denied and stale never reach the engine
	bp, eng := &fakeBlueprints{brief: "null"}, &fakeEngine{}
	if _, err := NewBlueprintPlanEditor(bp, eng, &fakeAuthz{deny: true}).Edit(context.Background(), "u", "t", bpID, ops, 1); !errors.Is(err, entity.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if _, err := NewBlueprintPlanEditor(bp, eng, &fakeAuthz{}).Edit(context.Background(), "u", "t", bpID, ops, 9); !errors.Is(err, entity.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	// another tenant's blueprint is not found, before any engine call
	if _, err := NewBlueprintPlanEditor(bp, eng, &fakeAuthz{}).Edit(context.Background(), "u", "other", bpID, ops, 1); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	// a plan with a brief is edited through the brief
	gen := &fakeBlueprints{brief: `{"title":"x"}`}
	if _, err := NewBlueprintPlanEditor(gen, eng, &fakeAuthz{}).Edit(context.Background(), "u", "t", bpID, ops, 1); !errors.Is(err, entity.ErrValidation) {
		t.Fatalf("want validation, got %v", err)
	}
	// bad bodies are refused before anything else
	for _, bad := range []json.RawMessage{nil, json.RawMessage(`{`), json.RawMessage(`"x"`), json.RawMessage(`null`), json.RawMessage(make([]byte, maxPlanEditBytes+1))} {
		if _, err := NewBlueprintPlanEditor(bp, eng, &fakeAuthz{}).Edit(context.Background(), "u", "t", bpID, bad, 1); !errors.Is(err, entity.ErrValidation) {
			t.Fatalf("want validation for %q, got %v", bad[:min(len(bad), 5)], err)
		}
	}
	// an engine refusal stores nothing
	rej := &fakeEngine{err: entity.Invalid("cannot move")}
	if _, err := NewBlueprintPlanEditor(bp, rej, &fakeAuthz{}).Edit(context.Background(), "u", "t", bpID, ops, 1); err == nil {
		t.Fatal("want error")
	}
	if len(eng.calls) != 0 || len(bp.appended) != 0 {
		t.Fatalf("engine %v stored %v", eng.calls, bp.appended)
	}
}

func TestExportPatchesTheOriginalWithBaseAndCurrentModels(t *testing.T) {
	eng := &fakeEngine{}
	f, err := NewBlueprintOriginalExporter(withOriginal{fakeBlueprints: &fakeBlueprints{brief: "null"}}, eng, &fakeAuthz{}).Export(context.Background(), "u", "t", bpID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(f.Data) != "PATCHED" || f.Name != "plan-edited.dxf" {
		t.Fatalf("file %+v", f)
	}
	// the engine sees the file under a neutral name, the original bytes and both models; no ids
	if len(eng.calls) != 1 || eng.calls[0] != `PatchDXF original.dxf ORIGINAL {"a":1} {"a":1}` {
		t.Fatalf("engine calls %q", eng.calls)
	}
}

func TestExportRefusesAVersionOlderThanTheFile(t *testing.T) {
	eng := &fakeEngine{}
	bp := withOriginal{fakeBlueprints: &fakeBlueprints{brief: "null"}, version: 3}
	if _, err := NewBlueprintOriginalExporter(bp, eng, &fakeAuthz{}).Export(context.Background(), "u", "t", bpID, 2); !errors.Is(err, entity.ErrValidation) {
		t.Fatalf("want validation, got %v", err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("engine calls %v", eng.calls)
	}
}

func TestExportRefusals(t *testing.T) {
	eng := &fakeEngine{}
	bp := withOriginal{fakeBlueprints: &fakeBlueprints{brief: "null"}}
	if _, err := NewBlueprintOriginalExporter(bp, eng, &fakeAuthz{deny: true}).Export(context.Background(), "u", "t", bpID, 0); !errors.Is(err, entity.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if _, err := NewBlueprintOriginalExporter(withOriginal{fakeBlueprints: bp.fakeBlueprints, none: true}, eng, &fakeAuthz{}).Export(context.Background(), "u", "t", bpID, 0); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	if _, err := NewBlueprintOriginalExporter(bp, eng, &fakeAuthz{}).Export(context.Background(), "u", "other", bpID, 0); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("want not found for another tenant, got %v", err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("engine called %v", eng.calls)
	}
}

func TestOriginalDefaultsToTheNewestFileNotTheLatestEdit(t *testing.T) {
	bp := withOriginal{fakeBlueprints: &fakeBlueprints{brief: "null"}, version: 3}
	o, err := NewBlueprintOriginalGetter(bp, &fakeAuthz{}).GetOriginal(context.Background(), "u", "t", bpID, 0)
	if err != nil || o.Version != 3 {
		t.Fatalf("original %+v, err %v", o, err)
	}
	if _, err := NewBlueprintOriginalGetter(bp, &fakeAuthz{deny: true}).GetOriginal(context.Background(), "u", "t", bpID, 0); !errors.Is(err, entity.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
}
