package service

import (
	"context"
	"encoding/json"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// loadEstimateSession returns the session when it exists in the header tenant (404 otherwise, so another
// tenant's session id reveals nothing).
func loadEstimateSession(ctx context.Context, sessions gateway.EstimateSessionGateway, tenantID, id string) (entity.EstimateSession, error) {
	if err := entity.RequireUUID("id", id); err != nil {
		return entity.EstimateSession{}, err
	}
	s, err := sessions.Get(ctx, id)
	if err != nil {
		return entity.EstimateSession{}, err
	}
	if s.TenantID != tenantID {
		return entity.EstimateSession{}, entity.ErrNotFound
	}
	return s, nil
}

// latestEstimate is the newest estimate of the session's blueprint made in this session, or nil.
func latestEstimate(ctx context.Context, estimates gateway.BlueprintEstimateGateway, s entity.EstimateSession) (*entity.BlueprintEstimate, error) {
	all, err := estimates.ListByBlueprint(ctx, s.BlueprintID)
	if err != nil {
		return nil, err
	}
	for _, e := range all {
		if e.SessionID == s.ID {
			full, err := estimates.Get(ctx, s.BlueprintID, e.ID)
			if err != nil {
				return nil, err
			}
			return &full, nil
		}
	}
	return nil, nil
}

// engineTurnState is the part of the engine's state Go reads: the status column mirrors it.
func turnStatus(state json.RawMessage) string {
	var s struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(state, &s) != nil {
		return ""
	}
	return s.Status
}
