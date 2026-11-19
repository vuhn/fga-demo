package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type estimateGetter struct {
	blueprints gateway.BlueprintGateway
	estimates  gateway.BlueprintEstimateGateway
	authz      usecase.Authorizer
}

func NewEstimateGetter(blueprints gateway.BlueprintGateway, estimates gateway.BlueprintEstimateGateway, authz usecase.Authorizer) usecase.EstimateGetter {
	return &estimateGetter{blueprints: blueprints, estimates: estimates, authz: authz}
}

// Get returns a finished estimate. It derives from the plan geometry, which can_view already exposes, so
// reading needs can_view (404 when the caller cannot see the blueprint).
func (s *estimateGetter) Get(ctx context.Context, actor, tenantID, blueprintID, id string) (model.BlueprintEstimate, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, blueprintID); err != nil {
		return model.BlueprintEstimate{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", model.ObjectTypeBlueprint, blueprintID); err != nil {
		return model.BlueprintEstimate{}, err
	}
	if err := model.RequireUUID("id", id); err != nil {
		return model.BlueprintEstimate{}, err
	}
	return s.estimates.Get(ctx, blueprintID, id)
}

type estimateLister struct {
	blueprints gateway.BlueprintGateway
	estimates  gateway.BlueprintEstimateGateway
	authz      usecase.Authorizer
}

func NewEstimateLister(blueprints gateway.BlueprintGateway, estimates gateway.BlueprintEstimateGateway, authz usecase.Authorizer) usecase.EstimateLister {
	return &estimateLister{blueprints: blueprints, estimates: estimates, authz: authz}
}

func (s *estimateLister) List(ctx context.Context, actor, tenantID, blueprintID string) ([]model.BlueprintEstimate, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, blueprintID); err != nil {
		return nil, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", model.ObjectTypeBlueprint, blueprintID); err != nil {
		return nil, err
	}
	return s.estimates.ListByBlueprint(ctx, blueprintID)
}

// estimateTotals reads the three totals (VND, VAT included) out of the engine's estimate JSON.
func estimateTotals(raw json.RawMessage) (low, base, high int64, err error) {
	var e struct {
		Total map[string]float64 `json:"total"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return 0, 0, 0, err
	}
	l, b, h := e.Total["low"], e.Total["base"], e.Total["high"]
	if l < 0 || b < 0 || h < 0 || h < l {
		return 0, 0, 0, fmt.Errorf("totals out of order")
	}
	return int64(l), int64(b), int64(h), nil
}
