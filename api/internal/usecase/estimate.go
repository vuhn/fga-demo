package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// EstimateSessionDetail is a session with its messages and the newest estimate it produced (nil before any).
type EstimateSessionDetail struct {
	Session  entity.EstimateSession
	Messages []entity.EstimateMessage
	Estimate *entity.BlueprintEstimate
}

// EstimateSessionCreator starts a chat about the blueprint's current version. Requires can_edit (it triggers paid
// LLM and search calls).
type EstimateSessionCreator interface {
	Create(ctx context.Context, actor, tenantID, blueprintID string) (EstimateSessionDetail, error)
}

// EstimateSessionMessenger runs one agent turn. Every refusal (validation, Check, turn cap, engine down) is
// returned before the first onEvent call, so the caller can still answer with a plain status. Nothing is
// stored unless the turn completes: an abort or an engine error leaves the session as it was.
type EstimateSessionMessenger interface {
	Send(ctx context.Context, actor, tenantID, sessionID, message string, overrides map[string]float64, onEvent func(entity.EstimateEvent)) (EstimateSessionDetail, error)
}

type EstimateSessionGetter interface {
	Get(ctx context.Context, actor, tenantID, sessionID string) (EstimateSessionDetail, error)
}

type EstimateSessionLister interface {
	List(ctx context.Context, actor, tenantID, blueprintID string) ([]entity.EstimateSession, error)
}

// EstimateGetter and EstimateLister read finished estimates; they need can_view only, like the plan summary.
type EstimateGetter interface {
	Get(ctx context.Context, actor, tenantID, blueprintID, id string) (entity.BlueprintEstimate, error)
}

type EstimateLister interface {
	List(ctx context.Context, actor, tenantID, blueprintID string) ([]entity.BlueprintEstimate, error)
}
