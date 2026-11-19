package agent

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/grpcutil"
	agentv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/agent/v1"
	commonv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/common/v1"
)

// fake is an in-process Agent; each test sets only the handler it needs.
type fake struct {
	agentv1.UnimplementedAgentServiceServer
	brief  func(context.Context, *agentv1.BriefFromTextRequest) (*agentv1.BriefFromTextResponse, error)
	revise func(*agentv1.ReviseRequest, agentv1.AgentService_ReviseServer) error
	plan   func(*agentv1.RevisePlanRequest, agentv1.AgentService_RevisePlanServer) error
	review func(*agentv1.ReviewRequest, agentv1.AgentService_ReviewServer) error
}

func (f *fake) BriefFromText(ctx context.Context, r *agentv1.BriefFromTextRequest) (*agentv1.BriefFromTextResponse, error) {
	return f.brief(ctx, r)
}
func (f *fake) Revise(r *agentv1.ReviseRequest, s agentv1.AgentService_ReviseServer) error {
	return f.revise(r, s)
}
func (f *fake) RevisePlan(r *agentv1.RevisePlanRequest, s agentv1.AgentService_RevisePlanServer) error {
	return f.plan(r, s)
}
func (f *fake) Review(r *agentv1.ReviewRequest, s agentv1.AgentService_ReviewServer) error {
	return f.review(r, s)
}

func serve(t *testing.T, f *fake) *Client {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	cc, err := grpcutil.Dial("passthrough:///bufnet", time.Minute,
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cc.Close(); srv.Stop() })
	return New(cc)
}

func progress(stage string) *agentv1.BlueprintEvent {
	return &agentv1.BlueprintEvent{Event: &agentv1.BlueprintEvent_Progress{Progress: &agentv1.Progress{Kind: "step", Stage: stage, Message: "m-" + stage}}}
}

func result(r *agentv1.BlueprintOutcome) *agentv1.BlueprintEvent {
	return &agentv1.BlueprintEvent{Event: &agentv1.BlueprintEvent_Result{Result: r}}
}

func inBandError(status int32, kind, msg string) *agentv1.BlueprintEvent {
	return &agentv1.BlueprintEvent{Event: &agentv1.BlueprintEvent_Error{Error: &agentv1.Error{Status: status, Error: kind, Message: msg}}}
}

// refuse is an INVALID_ARGUMENT with an ErrorDetail in the trailer, as the Agent sends it before a stream opens.
func refuse(ctx context.Context, code commonv1.ErrorCode, msg string, dep string) error {
	b, _ := proto.Marshal(&commonv1.ErrorDetail{Code: code, Message: msg, Dependency: dep})
	_ = grpc.SetTrailer(ctx, metadata.Pairs(grpcutil.ErrorDetailKey, string(b)))
	if dep != "" {
		return status.Error(codes.Unavailable, "down")
	}
	return status.Error(codes.InvalidArgument, "refused")
}

var outcomeMsg = &agentv1.BlueprintOutcome{BriefJson: []byte(`{"b":2}`), ModelJson: []byte(`{"m":2}`), Ok: true, Issues: []string{"i"},
	Notes: []string{"n"}, Attempts: 2, Checks: []*commonv1.Check{{Rule: "r1", Severity: "error", Message: "m", RoomId: "room1"}}}

func TestBriefFromTextSendsThePromptAndAnIdAndNothingElse(t *testing.T) {
	var md metadata.MD
	f := &fake{brief: func(ctx context.Context, r *agentv1.BriefFromTextRequest) (*agentv1.BriefFromTextResponse, error) {
		md, _ = metadata.FromIncomingContext(ctx)
		if r.Prompt != "a small house" || r.Enhance {
			t.Errorf("request = %+v", r)
		}
		return &agentv1.BriefFromTextResponse{BriefJson: []byte(`{"title":"T"}`), Notes: []string{"n"}}, nil
	}}
	d, err := serve(t, f).BriefFromText(grpcutil.WithRequestID(context.Background(), "req-1"), "a small house")
	if err != nil || string(d.Brief) != `{"title":"T"}` || len(d.Notes) != 1 || d.Notes[0] != "n" {
		t.Fatalf("draft = %+v, %v", d, err)
	}
	if got := md.Get(grpcutil.RequestIDKey); len(got) != 1 || got[0] != "req-1" {
		t.Errorf("request id = %v", got)
	}
	for k := range md { // only the request id and gRPC's own headers: nothing names a tenant or a person
		switch k {
		case grpcutil.RequestIDKey, ":authority", "content-type", "user-agent", "grpc-accept-encoding":
		default:
			t.Errorf("metadata %q must not be sent", k)
		}
	}
}

func TestReviseStreamReportsEachStepThenReturnsTheOutcome(t *testing.T) {
	f := &fake{revise: func(r *agentv1.ReviseRequest, s agentv1.AgentService_ReviseServer) error {
		if string(r.BriefJson) != `{"b":1}` || string(r.PreviousModelJson) != `{"m":1}` || r.Message != "bigger" || len(r.History) != 1 || r.History[0] != "earlier" {
			t.Errorf("request = %+v", r)
		}
		for _, st := range []string{"ask_model", "ops"} {
			_ = s.Send(progress(st))
		}
		return s.Send(result(outcomeMsg))
	}}
	var got []model.BlueprintProgress
	out, err := serve(t, f).ReviseStream(context.Background(), []byte(`{"b":1}`), []byte(`{"m":1}`), "bigger", []string{"earlier"},
		func(p model.BlueprintProgress) { got = append(got, p) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Stage != "ask_model" || got[1].Stage != "ops" || got[0].Kind != "step" || got[0].Message != "m-ask_model" {
		t.Errorf("progress = %+v", got)
	}
	if string(out.Brief) != `{"b":2}` || string(out.Model) != `{"m":2}` || !out.OK || out.Issues[0] != "i" || out.Notes[0] != "n" {
		t.Errorf("outcome = %+v", out)
	}
	if !strings.Contains(string(out.Checks), `"room_id":"room1"`) {
		t.Errorf("checks = %s", out.Checks)
	}
}

func TestTheSyncVariantsCollectTheStream(t *testing.T) {
	f := &fake{
		revise: func(_ *agentv1.ReviseRequest, s agentv1.AgentService_ReviseServer) error {
			_ = s.Send(progress("ask_model"))
			return s.Send(result(outcomeMsg))
		},
		plan: func(r *agentv1.RevisePlanRequest, s agentv1.AgentService_RevisePlanServer) error {
			if string(r.ModelJson) != `{"p":1}` || r.Message != "rename" {
				t.Errorf("plan request = %+v", r)
			}
			return s.Send(result(&agentv1.BlueprintOutcome{ModelJson: []byte(`{"p":2}`), Ok: true}))
		},
		review: func(r *agentv1.ReviewRequest, s agentv1.AgentService_ReviewServer) error {
			if r.MaxRounds != 3 || string(r.BriefJson) != `{"b":1}` {
				t.Errorf("review request = %+v", r)
			}
			return s.Send(result(&agentv1.BlueprintOutcome{BriefJson: []byte(`{"b":3}`), Ok: false, Issues: []string{"left"}}))
		},
	}
	c := serve(t, f)
	if out, err := c.Revise(context.Background(), nil, nil, "x", nil); err != nil || string(out.Model) != `{"m":2}` {
		t.Errorf("Revise = %+v, %v", out, err)
	}
	if out, err := c.RevisePlan(context.Background(), []byte(`{"p":1}`), "rename", nil); err != nil || string(out.Model) != `{"p":2}` || out.Brief != nil {
		t.Errorf("RevisePlan = %+v, %v", out, err)
	}
	if out, err := c.Review(context.Background(), []byte(`{"b":1}`), nil, nil); err != nil || string(out.Brief) != `{"b":3}` || out.OK || out.Issues[0] != "left" {
		t.Errorf("Review = %+v, %v", out, err)
	}
}

func TestAStatusBeforeTheStreamOpensIsAPlainError(t *testing.T) {
	f := &fake{revise: func(_ *agentv1.ReviseRequest, s agentv1.AgentService_ReviseServer) error {
		return refuse(s.Context(), commonv1.ErrorCode_ERROR_CODE_EDIT_ERROR, "message is empty", "")
	}}
	called := false
	_, err := serve(t, f).ReviseStream(context.Background(), nil, nil, " ", nil, func(model.BlueprintProgress) { called = true })
	if !errors.Is(err, model.ErrValidation) || !strings.Contains(err.Error(), "message is empty") || called {
		t.Errorf("err = %v, progress called = %v", err, called)
	}
	down := &fake{revise: func(_ *agentv1.ReviseRequest, s agentv1.AgentService_ReviseServer) error {
		return refuse(s.Context(), commonv1.ErrorCode_ERROR_CODE_DEPENDENCY_UNAVAILABLE, "ollama down", "llm")
	}}
	_, err = serve(t, down).Revise(context.Background(), nil, nil, "x", nil)
	if !errors.Is(err, model.ErrEngineUnavailable) || !strings.Contains(err.Error(), "ollama down") {
		t.Errorf("err = %v", err)
	}
}

func TestAnErrorEventAfterTheFirstMessageIsMappedLikeAStatus(t *testing.T) {
	cases := []struct {
		status int32
		kind   string
		want   error
	}{
		{422, "invalid_edit", model.ErrValidation},
		{502, "llm_unavailable", model.ErrEngineUnavailable},
		{502, "engine_unavailable", model.ErrEngineUnavailable},
		{500, "internal", model.ErrEngineUnavailable},
	}
	for _, c := range cases {
		f := &fake{revise: func(_ *agentv1.ReviseRequest, s agentv1.AgentService_ReviseServer) error {
			_ = s.Send(progress("ask_model"))
			return s.Send(inBandError(c.status, c.kind, "boom"))
		}}
		var steps int
		_, err := serve(t, f).ReviseStream(context.Background(), nil, nil, "x", nil, func(model.BlueprintProgress) { steps++ })
		if !errors.Is(err, c.want) || !strings.Contains(err.Error(), "boom") || steps != 1 {
			t.Errorf("%d %s: err = %v, steps = %d", c.status, c.kind, err, steps)
		}
	}
}

func TestAStreamThatEndsWithoutAResultIsAnOutage(t *testing.T) {
	f := &fake{review: func(_ *agentv1.ReviewRequest, s agentv1.AgentService_ReviewServer) error { return s.Send(progress("review")) }}
	_, err := serve(t, f).Review(context.Background(), nil, nil, nil)
	if !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("err = %v", err)
	}
	broken := &fake{review: func(_ *agentv1.ReviewRequest, _ agentv1.AgentService_ReviewServer) error {
		return status.Error(codes.Unavailable, "connection lost")
	}}
	if _, err := serve(t, broken).Review(context.Background(), nil, nil, nil); !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestAnAgentThatIsDownIsAnOutage(t *testing.T) {
	cc, err := grpcutil.Dial("passthrough:///nowhere", time.Second,
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return nil, errors.New("refused") }))
	if err != nil {
		t.Fatal(err)
	}
	defer cc.Close()
	c := New(cc)
	if _, err := c.BriefFromText(context.Background(), "x"); !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("BriefFromText err = %v", err)
	}
	if _, err := c.Revise(context.Background(), nil, nil, "x", nil); !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("Revise err = %v", err)
	}
}

func TestACallerThatLeavesCancelsTheAgentsRun(t *testing.T) {
	left := make(chan struct{})
	f := &fake{revise: func(_ *agentv1.ReviseRequest, s agentv1.AgentService_ReviseServer) error {
		_ = s.Send(progress("ask_model"))
		<-s.Context().Done() // the Agent's run stops when the stream is cancelled
		close(left)
		return s.Context().Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	_, err := serve(t, f).ReviseStream(ctx, nil, nil, "x", nil, func(model.BlueprintProgress) { cancel() })
	if !errors.Is(err, model.ErrEngineUnavailable) {
		t.Errorf("err = %v", err)
	}
	select {
	case <-left:
	case <-time.After(5 * time.Second):
		t.Fatal("the Agent was not told the caller left")
	}
}
