package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/infrastructure/grpcutil"
	agentv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/agent/v1"
	commonv1 "github.com/vuhn/fga-demo/proto/go/fga/demo/common/v1"
)

var _ gateway.EstimateAgent = (*Client)(nil)

// EstimateCeiling bounds a whole turn. A turn that researches many prices on a local model runs for many
// minutes, so there is no per-call timeout; the request context ends it when the browser disconnects. The Agent
// has its own 30 minute ceiling and answers with a 504 `timeout` event, so this one is a little longer and only
// catches an Agent that is stuck.
const EstimateCeiling = 31 * time.Minute

// EstimateTurn runs one turn over the Agent's stream. Every event except the last is passed to onEvent in the
// JSON shape the browser already reads (progress {kind, stage, message}; token {text}; fact {name, value,
// source}; question {id, text, options, allow_unknown}; rate; estimate). The Done event is the outcome. A
// refusal before the first message is a plain error and onEvent is never called; an Error event after it is
// mapped the same way (4xx -> ErrValidation, else ErrEngineUnavailable). Leaving ctx cancels the stream, which
// stops the Agent's turn.
func (c *Client) EstimateTurn(ctx context.Context, p model.EstimateTurnParams, onEvent func(model.EstimateEvent)) (model.EstimateTurn, error) {
	ctx, cancel := context.WithTimeout(ctx, EstimateCeiling)
	defer cancel()
	chat := make([]*agentv1.ChatMessage, 0, len(p.Messages))
	for _, m := range p.Messages {
		chat = append(chat, &agentv1.ChatMessage{Role: m.Role, Content: m.Content})
	}
	var tr metadata.MD
	s, err := c.rpc.EstimateTurn(ctx, &agentv1.EstimateTurnRequest{ModelJson: p.Model, StateJson: p.State, Message: p.Message,
		Messages: chat, RateOverrides: p.Overrides}, grpcTrailer(&tr))
	if err != nil {
		return model.EstimateTurn{}, grpcutil.FromStatus(service, err, grpcutil.Trailer(tr))
	}
	for {
		ev, err := s.Recv()
		if errors.Is(err, io.EOF) {
			return model.EstimateTurn{}, fmt.Errorf("%w: agent: the stream ended without a result", model.ErrEngineUnavailable)
		}
		if err != nil {
			if ctx.Err() != nil {
				return model.EstimateTurn{}, ctx.Err()
			}
			return model.EstimateTurn{}, grpcutil.FromStatus(service, err, grpcutil.Trailer(tr))
		}
		switch e := ev.Event.(type) {
		case *agentv1.EstimateEvent_Done_:
			return model.EstimateTurn{State: raw(e.Done.StateJson), Status: e.Done.Status, Reply: e.Done.Reply, Estimate: raw(e.Done.EstimateJson)}, nil
		case *agentv1.EstimateEvent_Error:
			return model.EstimateTurn{}, inBand(e.Error)
		default:
			if onEvent != nil {
				if out, ok := browserEvent(ev); ok {
					onEvent(out)
				}
			}
		}
	}
}

// browserEvent is one stream event in the shape the web panel reads, or false for an event with nothing to show.
func browserEvent(ev *agentv1.EstimateEvent) (model.EstimateEvent, bool) {
	switch e := ev.Event.(type) {
	case *agentv1.EstimateEvent_Progress:
		return jsonEvent("progress", map[string]string{"kind": e.Progress.Kind, "stage": e.Progress.Stage, "message": e.Progress.Message})
	case *agentv1.EstimateEvent_Token_:
		return jsonEvent("token", map[string]string{"text": e.Token.Text})
	case *agentv1.EstimateEvent_Fact:
		return jsonEvent("fact", map[string]any{"name": e.Fact.Name, "value": factValue(e.Fact.Name, e.Fact.Value), "source": e.Fact.Source})
	case *agentv1.EstimateEvent_Question:
		options := e.Question.Options
		if options == nil {
			options = []string{}
		}
		return jsonEvent("question", map[string]any{"id": e.Question.Id, "text": e.Question.Text, "options": options,
			"allow_unknown": e.Question.AllowUnknown})
	case *agentv1.EstimateEvent_Rate:
		return jsonEvent("rate", rateJSON(e.Rate))
	case *agentv1.EstimateEvent_EstimateJson:
		return model.EstimateEvent{Name: "estimate", Data: e.EstimateJson}, true
	}
	return model.EstimateEvent{}, false
}

func jsonEvent(name string, v any) (model.EstimateEvent, bool) {
	b, err := json.Marshal(v)
	return model.EstimateEvent{Name: name, Data: b}, err == nil
}

// factValue gives a fact back its type: the Agent sends every value as text. A room's state (scope.<id>) stays
// text; "true" and "false" are booleans and a number is a number, as the browser has always been sent.
func factValue(name, v string) any {
	if strings.HasPrefix(name, "scope.") {
		return v
	}
	switch v {
	case "true":
		return true
	case "false":
		return false
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}

type rateSourceJSON struct {
	URL         string `json:"url"`
	Quote       string `json:"quote"`
	RetrievedAt string `json:"retrieved_at"`
}

func rateJSON(r *commonv1.RateResult) map[string]any {
	sources := make([]rateSourceJSON, 0, len(r.Sources))
	for _, s := range r.Sources {
		sources = append(sources, rateSourceJSON{URL: s.Url, Quote: s.Quote, RetrievedAt: s.RetrievedAt})
	}
	return map[string]any{"code": r.Code, "low": r.Low, "base": r.Base, "high": r.High, "unit": r.Unit, "currency": r.Currency,
		"confidence": r.Confidence, "region": r.Region, "quality": r.Quality, "note": r.Note, "sources": sources}
}
