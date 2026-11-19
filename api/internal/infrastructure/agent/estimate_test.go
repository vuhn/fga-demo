package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/grpcutil"
	agentv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/agent/v1"
	commonv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func progressE() *agentv1.EstimateEvent {
	return &agentv1.EstimateEvent{Event: &agentv1.EstimateEvent_Progress{Progress: &agentv1.Progress{Kind: "step", Stage: "research", Message: "looking"}}}
}

func doneE(d *agentv1.EstimateEvent_Done) *agentv1.EstimateEvent {
	return &agentv1.EstimateEvent{Event: &agentv1.EstimateEvent_Done_{Done: d}}
}

func errE(status int32, kind, msg string) *agentv1.EstimateEvent {
	return &agentv1.EstimateEvent{Event: &agentv1.EstimateEvent_Error{Error: &agentv1.Error{Status: status, Error: kind, Message: msg}}}
}

func decode(t *testing.T, ev model.EstimateEvent) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(ev.Data, &m); err != nil {
		t.Fatalf("%s data %s: %v", ev.Name, ev.Data, err)
	}
	return m
}

func TestEstimateTurnSendsTheRequestAndReturnsTheDoneEvent(t *testing.T) {
	f := &fake{turn: func(r *agentv1.EstimateTurnRequest, s agentv1.AgentService_EstimateTurnServer) error {
		if string(r.ModelJson) != `{"plan":1}` || string(r.StateJson) != `{"s":1}` || r.Message != "hi" || r.RateOverrides["paint"] != 9000 ||
			len(r.Messages) != 2 || r.Messages[1].Role != "assistant" || r.Messages[1].Content != "b" {
			t.Errorf("request = %+v", r)
		}
		return s.Send(doneE(&agentv1.EstimateEvent_Done{StateJson: []byte(`{"s":2}`), Status: "ready", Reply: "done text", EstimateJson: []byte(`{"total":1}`)}))
	}}
	turn, err := serve(t, f).EstimateTurn(context.Background(), []byte(`{"plan":1}`), []byte(`{"s":1}`), "hi",
		[]model.EstimateMessage{{Role: "user", Content: "a"}, {Role: "assistant", Content: "b"}}, map[string]float64{"paint": 9000}, nil)
	if err != nil || string(turn.State) != `{"s":2}` || turn.Status != "ready" || turn.Reply != "done text" || string(turn.Estimate) != `{"total":1}` {
		t.Fatalf("turn = %+v, %v", turn, err)
	}
}

func TestEveryLiveEventReachesTheBrowserInTheShapeItAlwaysRead(t *testing.T) {
	low, base := 1.0, 2.0
	f := &fake{turn: func(_ *agentv1.EstimateTurnRequest, s agentv1.AgentService_EstimateTurnServer) error {
		for _, e := range []*agentv1.EstimateEvent{
			progressE(),
			{Event: &agentv1.EstimateEvent_Token_{Token: &agentv1.EstimateEvent_Token{Text: "Hel"}}},
			{Event: &agentv1.EstimateEvent_Fact{Fact: &agentv1.EstimateEvent_FactEvent{Name: "mep_in_scope", Value: "true", Source: "user"}}},
			{Event: &agentv1.EstimateEvent_Fact{Fact: &agentv1.EstimateEvent_FactEvent{Name: "floor_height", Value: "2.85", Source: "assumed"}}},
			{Event: &agentv1.EstimateEvent_Fact{Fact: &agentv1.EstimateEvent_FactEvent{Name: "city", Value: "Hanoi", Source: "user"}}},
			{Event: &agentv1.EstimateEvent_Fact{Fact: &agentv1.EstimateEvent_FactEvent{Name: "scope.r1", Value: "empty", Source: "user"}}},
			{Event: &agentv1.EstimateEvent_Question{Question: &agentv1.EstimateQuestion{Id: "city", Text: "Where?"}}},
			{Event: &agentv1.EstimateEvent_Rate{Rate: &commonv1.RateResult{Code: "paint", Low: &low, Base: &base, Unit: "m2", Currency: "VND",
				Confidence: "ok", Sources: []*commonv1.RateSource{{Url: "https://x.test", Quote: "q", RetrievedAt: "2026-11-16"}}}}},
			{Event: &agentv1.EstimateEvent_EstimateJson{EstimateJson: []byte(`{"total":{"base":5}}`)}},
		} {
			_ = s.Send(e)
		}
		return s.Send(doneE(&agentv1.EstimateEvent_Done{Status: "ready", Reply: "r"}))
	}}
	var got []model.EstimateEvent
	if _, err := serve(t, f).EstimateTurn(context.Background(), nil, nil, "x", nil, nil, func(e model.EstimateEvent) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(got))
	for i, e := range got {
		names[i] = e.Name
	}
	if strings.Join(names, ",") != "progress,token,fact,fact,fact,fact,question,rate,estimate" {
		t.Fatalf("events = %v", names)
	}
	if p := decode(t, got[0]); p["kind"] != "step" || p["stage"] != "research" || p["message"] != "looking" {
		t.Errorf("progress = %v", p)
	}
	if decode(t, got[1])["text"] != "Hel" {
		t.Errorf("token = %s", got[1].Data)
	}
	if v := decode(t, got[2])["value"]; v != true {
		t.Errorf("a boolean fact must be a JSON boolean, got %v", v)
	}
	if v := decode(t, got[3])["value"]; v != 2.85 {
		t.Errorf("a number fact must be a JSON number, got %v", v)
	}
	if v := decode(t, got[4])["value"]; v != "Hanoi" {
		t.Errorf("a text fact stays text, got %v", v)
	}
	if f := decode(t, got[5]); f["value"] != "empty" || f["name"] != "scope.r1" || f["source"] != "user" {
		t.Errorf("a room's state stays text: %v", f)
	}
	if q := decode(t, got[6]); q["id"] != "city" || q["text"] != "Where?" || q["allow_unknown"] != false || len(q["options"].([]any)) != 0 {
		t.Errorf("question = %v (options must be [] not null)", q)
	}
	r := decode(t, got[7])
	src := r["sources"].([]any)[0].(map[string]any)
	if r["code"] != "paint" || r["low"] != 1.0 || r["base"] != 2.0 || r["high"] != nil || src["retrieved_at"] != "2026-11-16" || src["url"] != "https://x.test" {
		t.Errorf("rate = %v", r)
	}
	if string(got[8].Data) != `{"total":{"base":5}}` {
		t.Errorf("estimate = %s", got[8].Data)
	}
}

func TestAnEstimateTurnRefusedBeforeTheStreamOpensIsAPlainErrorWithNoEvents(t *testing.T) {
	f := &fake{turn: func(_ *agentv1.EstimateTurnRequest, s agentv1.AgentService_EstimateTurnServer) error {
		return refuse(s.Context(), commonv1.ErrorCode_ERROR_CODE_EDIT_ERROR, "the session state is not valid JSON", "")
	}}
	called := false
	_, err := serve(t, f).EstimateTurn(context.Background(), nil, []byte("x"), "hi", nil, nil, func(model.EstimateEvent) { called = true })
	if !errors.Is(err, model.ErrValidation) || !strings.Contains(err.Error(), "not valid JSON") || called {
		t.Errorf("err = %v, called = %v", err, called)
	}
	down := &fake{turn: func(_ *agentv1.EstimateTurnRequest, s agentv1.AgentService_EstimateTurnServer) error {
		return refuse(s.Context(), commonv1.ErrorCode_ERROR_CODE_DEPENDENCY_UNAVAILABLE, "engine down", "engine")
	}}
	if _, err := serve(t, down).EstimateTurn(context.Background(), nil, nil, "hi", nil, nil, nil); !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestAnErrorEventEndsTheTurnWithTheMatchingError(t *testing.T) {
	for _, c := range []struct {
		status int32
		kind   string
		want   error
	}{{422, "invalid_edit", model.ErrValidation}, {502, "llm_unavailable", model.ErrEngineUnavailable},
		{504, "timeout", model.ErrEngineUnavailable}, {500, "internal", model.ErrEngineUnavailable}} {
		f := &fake{turn: func(_ *agentv1.EstimateTurnRequest, s agentv1.AgentService_EstimateTurnServer) error {
			_ = s.Send(progressE())
			return s.Send(errE(c.status, c.kind, "boom"))
		}}
		steps := 0
		turn, err := serve(t, f).EstimateTurn(context.Background(), nil, nil, "x", nil, nil, func(model.EstimateEvent) { steps++ })
		if !errors.Is(err, c.want) || !strings.Contains(err.Error(), "boom") || steps != 1 || turn.Reply != "" {
			t.Errorf("%d %s: err = %v, steps = %d, turn = %+v", c.status, c.kind, err, steps, turn)
		}
	}
}

func TestAnEstimateStreamThatEndsWithoutDoneIsAnOutage(t *testing.T) {
	f := &fake{turn: func(_ *agentv1.EstimateTurnRequest, s agentv1.AgentService_EstimateTurnServer) error {
		return s.Send(progressE())
	}}
	if _, err := serve(t, f).EstimateTurn(context.Background(), nil, nil, "x", nil, nil, nil); !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("err = %v", err)
	}
	broken := &fake{turn: func(_ *agentv1.EstimateTurnRequest, _ agentv1.AgentService_EstimateTurnServer) error {
		return status.Error(codes.Unavailable, "lost")
	}}
	if _, err := serve(t, broken).EstimateTurn(context.Background(), nil, nil, "x", nil, nil, nil); !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestABrowserThatLeavesCancelsTheAgentsTurnAndTheCallerSeesTheCancel(t *testing.T) {
	left := make(chan struct{})
	f := &fake{turn: func(_ *agentv1.EstimateTurnRequest, s agentv1.AgentService_EstimateTurnServer) error {
		_ = s.Send(progressE())
		<-s.Context().Done()
		close(left)
		return s.Context().Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := serve(t, f).EstimateTurn(ctx, nil, nil, "x", nil, nil, func(model.EstimateEvent) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want the caller's cancel (the messenger writes nothing)", err)
	}
	select {
	case <-left:
	case <-time.After(5 * time.Second):
		t.Fatal("the Agent was not told the browser left")
	}
}

func TestAnEstimateTurnCarriesOnlyTheRequestId(t *testing.T) {
	var id string
	f := &fake{turn: func(_ *agentv1.EstimateTurnRequest, s agentv1.AgentService_EstimateTurnServer) error {
		id = strings.Join(mdOf(s.Context()), ",")
		return s.Send(doneE(&agentv1.EstimateEvent_Done{Status: "interviewing", Reply: "r"}))
	}}
	if _, err := serve(t, f).EstimateTurn(grpcutil.WithRequestID(context.Background(), "req-9"), nil, nil, "x", nil, nil, nil); err != nil || id != "req-9" {
		t.Errorf("err = %v, id = %q", err, id)
	}
}
