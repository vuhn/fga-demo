package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type estimateSessionGetter struct {
	blueprints gateway.BlueprintGateway
	sessions   gateway.EstimateSessionGateway
	authz      helper.Authorizer
}

func NewEstimateSessionGetter(blueprints gateway.BlueprintGateway, sessions gateway.EstimateSessionGateway, authz helper.Authorizer) usecase.EstimateSessionGetter {
	return &estimateSessionGetter{blueprints: blueprints, sessions: sessions, authz: authz}
}

// Get returns the blueprint's session with its messages (404 when it has none). The chat is can_edit; the
// Check fails as 404 so a viewer learns nothing about it (they read the finished estimates instead).
func (s *estimateSessionGetter) Get(ctx context.Context, in input.EstimateSessionGetter) (output.EstimateSessionGetter, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, in.TenantID, in.BlueprintID); err != nil {
		return output.EstimateSessionGetter{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_edit", model.ObjectTypeBlueprint, in.BlueprintID); err != nil {
		return output.EstimateSessionGetter{}, err
	}
	got, err := readEstimateSession(ctx, s.sessions, in.BlueprintID)
	return output.EstimateSessionGetter{EstimateSessionDetail: estimateSessionDetailOutput(got)}, err
}

// readEstimateSession is the read after a Check (or after our own write).
func readEstimateSession(ctx context.Context, sessions gateway.EstimateSessionGateway, blueprintID string) (estimateSessionDetail, error) {
	sess, err := sessions.Get(ctx, blueprintID)
	if err != nil {
		return estimateSessionDetail{}, err
	}
	msgs, err := sessions.Messages(ctx, blueprintID)
	if err != nil {
		return estimateSessionDetail{}, err
	}
	return estimateSessionDetail{Session: sess, Messages: msgs}, nil
}

// estimateSessionDetail is a blueprint's estimate session with its messages.
type estimateSessionDetail struct {
	Session  model.EstimateSession
	Messages []model.EstimateMessage
}
