package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// EstimateSessionGateway is the estimate_sessions and estimate_messages display record: one session per blueprint.
type EstimateSessionGateway interface {
	// Replace makes s the blueprint's session: any earlier one is reset (messages deleted, state cleared, epoch
	// raised) and s's version, creator and empty state stored, in one transaction. The blueprint's finished
	// estimates are not touched.
	Replace(ctx context.Context, s model.EstimateSession) error
	// Get returns the blueprint's session (ErrNotFound when it has none).
	Get(ctx context.Context, blueprintID string) (model.EstimateSession, error)
	// Messages returns the session's messages in order.
	Messages(ctx context.Context, blueprintID string) ([]model.EstimateMessage, error)
	// AppendTurn stores the user message, the assistant message and the new state, and the estimate when there is
	// one, in one transaction, only while the session's epoch and seq are still those of s read before the turn
	// (ErrConflict and nothing written otherwise).
	AppendTurn(ctx context.Context, s model.EstimateSession, state []byte, user, assistant model.EstimateMessage, est *model.BlueprintEstimate) error
}

// BlueprintEstimateGateway reads the finished estimates (they are written by AppendTurn).
type BlueprintEstimateGateway interface {
	Get(ctx context.Context, blueprintID, id string) (model.BlueprintEstimate, error)
	// ListByBlueprint returns the estimates of a blueprint, newest first, without their result.
	ListByBlueprint(ctx context.Context, blueprintID string) ([]model.BlueprintEstimate, error)
}

// EstimateAgent is the estimate agent (an LLM chat that builds a cost estimate). Same rules as BlueprintAgent: callers Check first.
type EstimateAgent interface {
	// EstimateTurn runs one turn. messages are the recent chat (oldest first), overrides are user-entered unit
	// rates by rate code. Events are passed to onEvent as they arrive; an error returned before the first
	// event is the same as any engine refusal (the caller can still answer with a plain status).
	EstimateTurn(ctx context.Context, model, state []byte, message string, messages []model.EstimateMessage, overrides map[string]float64, onEvent func(model.EstimateEvent)) (model.EstimateTurn, error)
}
