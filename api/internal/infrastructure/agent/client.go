// Package agent is the gRPC client of the Agent (:8091), which owns every LLM call. It implements
// gateway.BlueprintAgent. The Agent is stateless, has no auth and never sees a tenant or a person: the services
// run their OpenFGA Checks before calling it. A refusal before the first stream message is a gRPC status
// (ErrValidation, or ErrEngineUnavailable when a dependency is down); after the first message it is an in-band
// Error event, mapped the same way.
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/grpcutil"
	agentv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/agent/v1"
)

// DefaultTimeout bounds a blueprint call (a model step takes tens of seconds on a laptop, a review several).
const DefaultTimeout = 5 * time.Minute

const service = "agent"

// reviewRounds is the most rounds a review may take (the Agent's own ceiling is the same).
const reviewRounds = 3

// Client calls the Agent over gRPC.
type Client struct {
	rpc agentv1.AgentServiceClient
}

var _ gateway.BlueprintAgent = (*Client)(nil)

// New wraps an open connection (see Dial).
func New(cc grpc.ClientConnInterface) *Client {
	return &Client{rpc: agentv1.NewAgentServiceClient(cc)}
}

// Dial connects to addr. Connecting is lazy: an Agent that is down surfaces as ErrEngineUnavailable on the first
// call, not at start-up.
func Dial(addr string) (*Client, *grpc.ClientConn, error) {
	cc, err := grpcutil.Dial(addr, DefaultTimeout)
	if err != nil {
		return nil, nil, err
	}
	return New(cc), cc, nil
}

func (c *Client) BriefFromText(ctx context.Context, prompt string) (model.BlueprintDraft, error) {
	r, err := grpcutil.Call(service, func(tr *metadata.MD) (*agentv1.BriefFromTextResponse, error) {
		return c.rpc.BriefFromText(ctx, &agentv1.BriefFromTextRequest{Prompt: prompt}, grpc.Trailer(tr))
	})
	if err != nil {
		return model.BlueprintDraft{}, err
	}
	return model.BlueprintDraft{Brief: raw(r.BriefJson), Notes: r.Notes}, nil
}

func (c *Client) Revise(ctx context.Context, brief, previous []byte, message string, history []string) (model.BlueprintOutcome, error) {
	return c.ReviseStream(ctx, brief, previous, message, history, nil)
}

func (c *Client) ReviseStream(ctx context.Context, brief, previous []byte, message string, history []string, onProgress func(model.BlueprintProgress)) (model.BlueprintOutcome, error) {
	return run(ctx, onProgress, func(ctx context.Context, tr *metadata.MD) (stream, error) {
		return c.rpc.Revise(ctx, &agentv1.ReviseRequest{BriefJson: brief, PreviousModelJson: previous, Message: message, History: history}, grpc.Trailer(tr))
	})
}

func (c *Client) RevisePlan(ctx context.Context, plan []byte, message string, history []string) (model.BlueprintOutcome, error) {
	return c.RevisePlanStream(ctx, plan, message, history, nil)
}

func (c *Client) RevisePlanStream(ctx context.Context, plan []byte, message string, history []string, onProgress func(model.BlueprintProgress)) (model.BlueprintOutcome, error) {
	return run(ctx, onProgress, func(ctx context.Context, tr *metadata.MD) (stream, error) {
		return c.rpc.RevisePlan(ctx, &agentv1.RevisePlanRequest{ModelJson: plan, Message: message, History: history}, grpc.Trailer(tr))
	})
}

func (c *Client) Review(ctx context.Context, brief, previous []byte, history []string) (model.BlueprintOutcome, error) {
	return run(ctx, nil, func(ctx context.Context, tr *metadata.MD) (stream, error) {
		return c.rpc.Review(ctx, &agentv1.ReviewRequest{BriefJson: brief, PreviousModelJson: previous, History: history, MaxRounds: reviewRounds}, grpc.Trailer(tr))
	})
}

// stream is what the three blueprint RPCs return.
type stream interface {
	Recv() (*agentv1.BlueprintEvent, error)
}

// run opens a blueprint stream, passes each progress step to onProgress (nil: nobody listens) and returns the one
// result. A status before the first message and an Error event after it are mapped alike (see the package doc); a
// stream that ends without a result is an outage. Leaving ctx cancels the stream, which stops the Agent's run.
func run(ctx context.Context, onProgress func(model.BlueprintProgress), open func(context.Context, *metadata.MD) (stream, error)) (model.BlueprintOutcome, error) {
	ctx, cancel := grpcutil.WithDeadline(ctx, DefaultTimeout)
	defer cancel()
	var tr metadata.MD
	s, err := open(ctx, &tr)
	if err != nil {
		return model.BlueprintOutcome{}, grpcutil.FromStatus(service, err, grpcutil.Trailer(tr))
	}
	for {
		ev, err := s.Recv()
		if errors.Is(err, io.EOF) {
			return model.BlueprintOutcome{}, fmt.Errorf("%w: agent: the stream ended without a result", model.ErrEngineUnavailable)
		}
		if err != nil {
			return model.BlueprintOutcome{}, grpcutil.FromStatus(service, err, grpcutil.Trailer(tr))
		}
		switch e := ev.Event.(type) {
		case *agentv1.BlueprintEvent_Progress:
			if onProgress != nil {
				onProgress(model.BlueprintProgress{Kind: e.Progress.Kind, Stage: e.Progress.Stage, Message: e.Progress.Message})
			}
		case *agentv1.BlueprintEvent_Result:
			return outcome(e.Result), nil
		case *agentv1.BlueprintEvent_Error:
			return model.BlueprintOutcome{}, inBand(e.Error)
		}
	}
}

func outcome(r *agentv1.BlueprintOutcome) model.BlueprintOutcome {
	return model.BlueprintOutcome{Brief: raw(r.BriefJson), Model: raw(r.ModelJson), Checks: grpcutil.ChecksJSON(r.Checks),
		Issues: r.Issues, OK: r.Ok, Notes: r.Notes}
}

// inBand maps an Error event like the HTTP engine's error line: a 4xx is a refusal (ErrValidation), anything else an outage.
func inBand(e *agentv1.Error) error {
	if e.Status/100 == 4 {
		return model.Invalid("%s", e.Message)
	}
	return fmt.Errorf("%w: agent %d %s: %s", model.ErrEngineUnavailable, e.Status, e.Error, e.Message)
}

// raw keeps an empty JSON field absent (nil) rather than an empty-but-non-nil message.
func raw(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}
