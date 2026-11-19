package blueprintengine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

var _ gateway.EstimateAgent = (*Client)(nil)

const estimateTurnCeiling = 30 * time.Minute

// EstimateTurn runs one turn of the estimate agent over the engine's NDJSON stream. Every line is passed to
// onEvent (progress as {kind, stage, message}; token, fact, question, rate and estimate with their data). The
// done line is the outcome; an error line is mapped like a plain engine error. A refusal before the stream
// opens comes back from send, before any event.
func (c *Client) EstimateTurn(ctx context.Context, plan, state []byte, message string, messages []model.EstimateMessage, overrides map[string]float64, onEvent func(model.EstimateEvent)) (model.EstimateTurn, error) {
	history := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		history = append(history, map[string]string{"role": m.Role, "content": m.Content})
	}
	if overrides == nil {
		overrides = map[string]float64{}
	}
	payload, err := json.Marshal(map[string]any{"model": json.RawMessage(plan), "state": rawOr(state, "{}"), "message": message,
		"messages": history, "rate_overrides": overrides})
	if err != nil {
		return model.EstimateTurn{}, err
	}
	// A turn that researches many prices on a local model runs for many minutes: no fixed client timeout (the
	// request context ends it when the browser disconnects), only this ceiling.
	ctx, cancel := context.WithTimeout(ctx, estimateTurnCeiling)
	defer cancel()
	long := *c.HTTP
	long.Timeout = 0
	resp, err := c.sendWith(ctx, &long, "/v1/estimate/agent/turn", "application/json", bytes.NewReader(payload))
	if err != nil {
		return model.EstimateTurn{}, err
	}
	defer resp.Body.Close()
	r := bufio.NewReaderSize(resp.Body, 1<<20)
	for {
		line, rerr := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var ev struct {
				Event   string          `json:"event"`
				Kind    string          `json:"kind"`
				Stage   string          `json:"stage"`
				Message string          `json:"message"`
				Status  int             `json:"status"`
				Data    json.RawMessage `json:"data"`
			}
			if jerr := json.Unmarshal(line, &ev); jerr != nil {
				return model.EstimateTurn{}, fmt.Errorf("%w: bad engine stream: %v", model.ErrEngineUnavailable, jerr)
			}
			switch ev.Event {
			case "done":
				var out struct {
					State    json.RawMessage `json:"state"`
					Reply    string          `json:"reply"`
					Estimate json.RawMessage `json:"estimate"`
				}
				if jerr := json.Unmarshal(ev.Data, &out); jerr != nil {
					return model.EstimateTurn{}, fmt.Errorf("%w: bad engine response: %v", model.ErrEngineUnavailable, jerr)
				}
				return model.EstimateTurn{State: out.State, Reply: out.Reply, Estimate: out.Estimate}, nil
			case "error":
				if ev.Status/100 == 4 {
					return model.EstimateTurn{}, model.Invalid("%s", ev.Message)
				}
				return model.EstimateTurn{}, fmt.Errorf("%w: engine %d: %s", model.ErrEngineUnavailable, ev.Status, ev.Message)
			case "progress":
				if onEvent != nil {
					data, _ := json.Marshal(map[string]string{"kind": ev.Kind, "stage": ev.Stage, "message": ev.Message})
					onEvent(model.EstimateEvent{Name: "progress", Data: data})
				}
			default:
				if onEvent != nil && ev.Event != "" {
					onEvent(model.EstimateEvent{Name: ev.Event, Data: ev.Data})
				}
			}
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return model.EstimateTurn{}, ctx.Err()
			}
			return model.EstimateTurn{}, fmt.Errorf("%w: engine stream ended without a result", model.ErrEngineUnavailable)
		}
	}
}

func rawOr(b []byte, fallback string) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(b)
}
