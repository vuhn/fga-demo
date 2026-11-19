package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// EstimateSessionDetail is a blueprint's estimate session with its messages.
type EstimateSessionDetail struct {
	Session  model.EstimateSession
	Messages []model.EstimateMessage
}

// EstimateSessionCreator starts a chat about the blueprint's current version. A blueprint has one session:
// starting replaces the old one (its chat and answers), finished estimates stay. Requires can_edit (it triggers
// paid LLM and search calls).
type EstimateSessionCreator interface {
	Create(ctx context.Context, actor, tenantID, blueprintID string) (EstimateSessionDetail, error)
}

// EstimateSessionMessenger runs one agent turn. Every refusal (validation, Check, turn cap, engine down) is
// returned before the first onEvent call, so the caller can still answer with a plain status. Nothing is
// stored unless the turn completes: an abort or an engine error leaves the session as it was.
type EstimateSessionMessenger interface {
	Send(ctx context.Context, actor, tenantID, blueprintID, message string, overrides map[string]float64, onEvent func(model.EstimateEvent)) (EstimateSessionDetail, error)
}

type EstimateSessionGetter interface {
	Get(ctx context.Context, actor, tenantID, blueprintID string) (EstimateSessionDetail, error)
}

// EstimateGetter and EstimateLister read finished estimates; they need can_view only, like the plan summary.
type EstimateGetter interface {
	Get(ctx context.Context, actor, tenantID, blueprintID, id string) (model.BlueprintEstimate, error)
}

type EstimateLister interface {
	List(ctx context.Context, actor, tenantID, blueprintID string) ([]model.BlueprintEstimate, error)
}
