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

const bpID = "11111111-1111-1111-1111-111111111111"

// fakeBlueprints embeds the interface so only the methods a revision uses are written.
type fakeBlueprints struct {
	gateway.BlueprintGateway
	brief    string
	appended []entity.BlueprintVersion
}

func (f *fakeBlueprints) Get(context.Context, string) (entity.Blueprint, error) {
	return entity.Blueprint{ID: bpID, TenantID: "t", CurrentVersion: 1}, nil
}
func (f *fakeBlueprints) GetVersion(context.Context, string, int) (entity.BlueprintVersion, error) {
	return entity.BlueprintVersion{BlueprintID: bpID, Version: 1, Brief: json.RawMessage(f.brief), Model: json.RawMessage(`{"a":1}`)}, nil
}
func (f *fakeBlueprints) ListVersions(context.Context, string) ([]entity.BlueprintVersion, error) {
	return nil, nil
}
func (f *fakeBlueprints) AppendVersion(_ context.Context, v entity.BlueprintVersion, _ int) error {
	f.appended = append(f.appended, v)
	return nil
}

// fakeEngine implements gateway.BlueprintEngine (the deterministic operations).
type fakeEngine struct {
	gateway.BlueprintEngine
	calls   []string
	err     error
	noModel bool // CreatePlan: the brief cannot be built
}

func (e *fakeEngine) out() (entity.BlueprintOutcome, error) {
	return entity.BlueprintOutcome{Brief: json.RawMessage(`{"must":"be ignored"}`), Model: json.RawMessage(`{"b":2}`), OK: true}, e.err
}

// fakeAgent implements gateway.BlueprintAgent (the LLM operations).
type fakeAgent struct {
	gateway.BlueprintAgent
	calls []string
	err   error
}

func (e *fakeAgent) out() (entity.BlueprintOutcome, error) {
	return entity.BlueprintOutcome{Brief: json.RawMessage(`{"must":"be ignored"}`), Model: json.RawMessage(`{"b":2}`), OK: true}, e.err
}
func (e *fakeAgent) Revise(context.Context, []byte, []byte, string, []string) (entity.BlueprintOutcome, error) {
	e.calls = append(e.calls, "Revise")
	return e.out()
}
func (e *fakeAgent) RevisePlan(context.Context, []byte, string, []string) (entity.BlueprintOutcome, error) {
	e.calls = append(e.calls, "RevisePlan")
	return e.out()
}

type fakeAuthz struct {
	usecase.Authorizer
	deny bool
}

func (a *fakeAuthz) Require(context.Context, string, string, string, string) error {
	if a.deny {
		return entity.ErrForbidden
	}
	return nil
}

func TestReviseOnImportedPlanUsesPlanEditsAndKeepsBriefNull(t *testing.T) {
	bp, eng := &fakeBlueprints{brief: "null"}, &fakeAgent{}
	s := NewBlueprintReviser(bp, eng, &fakeAuthz{})
	if _, err := s.Revise(context.Background(), "u", "t", bpID, "rename it", 1); err != nil && len(bp.appended) == 0 {
		t.Fatal(err)
	}
	if len(eng.calls) != 1 || eng.calls[0] != "RevisePlan" {
		t.Fatalf("engine calls %v", eng.calls)
	}
	if len(bp.appended) != 1 || string(bp.appended[0].Brief) != "null" {
		t.Fatalf("stored %+v", bp.appended)
	}
}

func TestReviseOnBriefPlanStillUsesBrief(t *testing.T) {
	bp, eng := &fakeBlueprints{brief: `{"title":"x"}`}, &fakeAgent{}
	_, _ = NewBlueprintReviser(bp, eng, &fakeAuthz{}).Revise(context.Background(), "u", "t", bpID, "add a study", 1)
	if len(eng.calls) != 1 || eng.calls[0] != "Revise" {
		t.Fatalf("engine calls %v", eng.calls)
	}
}

func TestReviseImportedWritesNothingOnEngineError(t *testing.T) {
	bp, eng := &fakeBlueprints{brief: "null"}, &fakeAgent{err: entity.Invalid("not supported")}
	if _, err := NewBlueprintReviser(bp, eng, &fakeAuthz{}).Revise(context.Background(), "u", "t", bpID, "move wall", 1); err == nil {
		t.Fatal("want error")
	}
	if len(bp.appended) != 0 {
		t.Fatalf("wrote %v", bp.appended)
	}
}

func TestReviseDeniedOrStaleNeverReachesTheEngine(t *testing.T) {
	bp, eng := &fakeBlueprints{brief: "null"}, &fakeAgent{}
	s := NewBlueprintReviser(bp, eng, &fakeAuthz{deny: true})
	if _, err := s.Revise(context.Background(), "u", "t", bpID, "x", 1); !errors.Is(err, entity.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if _, err := NewBlueprintReviser(bp, eng, &fakeAuthz{}).Revise(context.Background(), "u", "t", bpID, "x", 7); !errors.Is(err, entity.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("engine called %v", eng.calls)
	}
}
