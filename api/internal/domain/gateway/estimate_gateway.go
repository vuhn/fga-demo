package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// EstimateSessionGateway is the estimate_sessions and estimate_messages display record: one session per blueprint.
type EstimateSessionGateway interface {
	// Replace makes s the blueprint's session: any earlier one is reset (messages deleted, state cleared, epoch
	// raised) and s's version, creator and empty state stored, in one transaction. The blueprint's finished
	// estimates are not touched.
	Replace(ctx context.Context, s entity.EstimateSession) error
	// Get returns the blueprint's session (ErrNotFound when it has none).
	Get(ctx context.Context, blueprintID string) (entity.EstimateSession, error)
	// Messages returns the session's messages in order.
	Messages(ctx context.Context, blueprintID string) ([]entity.EstimateMessage, error)
	// AppendTurn stores the user message, the assistant message and the new state, and the estimate when there is
	// one, in one transaction, only while the session's epoch and seq are still those of s read before the turn
	// (ErrConflict and nothing written otherwise).
	AppendTurn(ctx context.Context, s entity.EstimateSession, state []byte, user, assistant entity.EstimateMessage, est *entity.BlueprintEstimate) error
}

// BlueprintEstimateGateway reads the finished estimates (they are written by AppendTurn).
type BlueprintEstimateGateway interface {
	Get(ctx context.Context, blueprintID, id string) (entity.BlueprintEstimate, error)
	// ListByBlueprint returns the estimates of a blueprint, newest first, without their result.
	ListByBlueprint(ctx context.Context, blueprintID string) ([]entity.BlueprintEstimate, error)
}

// EstimateEngine is the engine's estimate agent. Same rules as BlueprintEngine: callers Check first.
type EstimateEngine interface {
	// EstimateTurn runs one turn. messages are the recent chat (oldest first), overrides are user-entered unit
	// rates by rate code. Events are passed to onEvent as they arrive; an error returned before the first
	// event is the same as any engine refusal (the caller can still answer with a plain status).
	EstimateTurn(ctx context.Context, model, state []byte, message string, messages []entity.EstimateMessage, overrides map[string]float64, onEvent func(entity.EstimateEvent)) (entity.EstimateTurn, error)
}
