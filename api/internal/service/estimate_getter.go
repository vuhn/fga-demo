package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type estimateGetter struct {
	blueprints gateway.BlueprintGateway
	estimates  gateway.BlueprintEstimateGateway
	authz      helper.Authorizer
}

func NewEstimateGetter(blueprints gateway.BlueprintGateway, estimates gateway.BlueprintEstimateGateway, authz helper.Authorizer) usecase.EstimateGetter {
	return &estimateGetter{blueprints: blueprints, estimates: estimates, authz: authz}
}

// Get returns a finished estimate. It derives from the plan geometry, which can_view already exposes, so
// reading needs can_view (404 when the caller cannot see the blueprint).
func (s *estimateGetter) Get(ctx context.Context, in input.EstimateGetter) (output.EstimateGetter, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.BlueprintID); err != nil {
		return output.EstimateGetter{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeBlueprint, in.BlueprintID); err != nil {
		return output.EstimateGetter{}, err
	}
	if err := model.RequireUUID("id", in.ID); err != nil {
		return output.EstimateGetter{}, err
	}
	got, err := s.estimates.Get(ctx, in.BlueprintID, in.ID)
	return output.EstimateGetter{BlueprintEstimate: blueprintEstimateOutput(got)}, err
}

type estimateLister struct {
	blueprints gateway.BlueprintGateway
	estimates  gateway.BlueprintEstimateGateway
	authz      helper.Authorizer
}

func NewEstimateLister(blueprints gateway.BlueprintGateway, estimates gateway.BlueprintEstimateGateway, authz helper.Authorizer) usecase.EstimateLister {
	return &estimateLister{blueprints: blueprints, estimates: estimates, authz: authz}
}

func (s *estimateLister) List(ctx context.Context, in input.EstimateLister) (output.EstimateLister, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.BlueprintID); err != nil {
		return output.EstimateLister{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeBlueprint, in.BlueprintID); err != nil {
		return output.EstimateLister{}, err
	}
	got, err := s.estimates.ListByBlueprint(ctx, in.BlueprintID)
	return output.EstimateLister{Items: mapOutput(got, blueprintEstimateOutput)}, err
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
