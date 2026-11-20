package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

const bpID = "11111111-1111-1111-1111-111111111111"

// fakeBlueprints embeds the interface so only the methods a revision uses are written.
type fakeBlueprints struct {
	gateway.BlueprintGateway
	brief    string
	appended []model.BlueprintVersion
}

func (f *fakeBlueprints) Get(context.Context, string) (model.Blueprint, error) {
	return model.Blueprint{ID: bpID, TenantID: "t", CurrentVersion: 1}, nil
}
func (f *fakeBlueprints) GetVersion(context.Context, string, int) (model.BlueprintVersion, error) {
	return model.BlueprintVersion{BlueprintID: bpID, Version: 1, Brief: json.RawMessage(f.brief), Model: json.RawMessage(`{"a":1}`)}, nil
}
func (f *fakeBlueprints) ListVersions(context.Context, string) ([]model.BlueprintVersion, error) {
	return nil, nil
}
func (f *fakeBlueprints) AppendVersion(_ context.Context, v model.BlueprintVersion, _ int) error {
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

func (e *fakeEngine) out() (model.BlueprintOutcome, error) {
	return model.BlueprintOutcome{Brief: json.RawMessage(`{"must":"be ignored"}`), Model: json.RawMessage(`{"b":2}`), OK: true}, e.err
}

// fakeAgent implements gateway.BlueprintAgent (the LLM operations).
type fakeAgent struct {
	gateway.BlueprintAgent
	calls []string
	err   error
}

func (e *fakeAgent) out() (model.BlueprintOutcome, error) {
	return model.BlueprintOutcome{Brief: json.RawMessage(`{"must":"be ignored"}`), Model: json.RawMessage(`{"b":2}`), OK: true}, e.err
}
func (e *fakeAgent) Revise(context.Context, model.ReviseBlueprintParams) (model.BlueprintOutcome, error) {
	e.calls = append(e.calls, "Revise")
	return e.out()
}
func (e *fakeAgent) RevisePlan(context.Context, model.RevisePlanParams) (model.BlueprintOutcome, error) {
	e.calls = append(e.calls, "RevisePlan")
	return e.out()
}

type fakeAuthz struct {
	helper.Authorizer
	deny bool
}

func (a *fakeAuthz) Require(context.Context, string, string, string, string) error {
	if a.deny {
		return model.ErrForbidden
	}
	return nil
}

func TestReviseOnImportedPlanUsesPlanEditsAndKeepsBriefNull(t *testing.T) {
	bp, eng := &fakeBlueprints{brief: "null"}, &fakeAgent{}
	s := NewBlueprintReviser(bp, eng, &fakeAuthz{})
	if _, err := s.Revise(context.Background(), input.BlueprintReviser{Actor: "u", TenantID: "t", ID: bpID, Message: "rename it", BaseVersion: 1}); err != nil && len(bp.appended) == 0 {
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
	_, _ = NewBlueprintReviser(bp, eng, &fakeAuthz{}).Revise(context.Background(), input.BlueprintReviser{Actor: "u", TenantID: "t", ID: bpID, Message: "add a study", BaseVersion: 1})
	if len(eng.calls) != 1 || eng.calls[0] != "Revise" {
		t.Fatalf("engine calls %v", eng.calls)
	}
}

func TestReviseImportedWritesNothingOnEngineError(t *testing.T) {
	bp, eng := &fakeBlueprints{brief: "null"}, &fakeAgent{err: model.Invalid("not supported")}
	if _, err := NewBlueprintReviser(bp, eng, &fakeAuthz{}).Revise(context.Background(), input.BlueprintReviser{Actor: "u", TenantID: "t", ID: bpID, Message: "move wall", BaseVersion: 1}); err == nil {
		t.Fatal("want error")
	}
	if len(bp.appended) != 0 {
		t.Fatalf("wrote %v", bp.appended)
	}
}

func TestReviseDeniedOrStaleNeverReachesTheEngine(t *testing.T) {
	bp, eng := &fakeBlueprints{brief: "null"}, &fakeAgent{}
	s := NewBlueprintReviser(bp, eng, &fakeAuthz{deny: true})
	if _, err := s.Revise(context.Background(), input.BlueprintReviser{Actor: "u", TenantID: "t", ID: bpID, Message: "x", BaseVersion: 1}); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if _, err := NewBlueprintReviser(bp, eng, &fakeAuthz{}).Revise(context.Background(), input.BlueprintReviser{Actor: "u", TenantID: "t", ID: bpID, Message: "x", BaseVersion: 7}); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	if len(eng.calls) != 0 {
		t.Fatalf("engine called %v", eng.calls)
	}
}
