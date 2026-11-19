package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type estimateSessionGetter struct {
	blueprints gateway.BlueprintGateway
	sessions   gateway.EstimateSessionGateway
	authz      usecase.Authorizer
}

func NewEstimateSessionGetter(blueprints gateway.BlueprintGateway, sessions gateway.EstimateSessionGateway, authz usecase.Authorizer) usecase.EstimateSessionGetter {
	return &estimateSessionGetter{blueprints: blueprints, sessions: sessions, authz: authz}
}

// Get returns the blueprint's session with its messages (404 when it has none). The chat is can_edit; the
// Check fails as 404 so a viewer learns nothing about it (they read the finished estimates instead).
func (s *estimateSessionGetter) Get(ctx context.Context, actor, tenantID, blueprintID string) (usecase.EstimateSessionDetail, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, blueprintID); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_edit", model.ObjectTypeBlueprint, blueprintID); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	return readEstimateSession(ctx, s.sessions, blueprintID)
}

// readEstimateSession is the read after a Check (or after our own write).
func readEstimateSession(ctx context.Context, sessions gateway.EstimateSessionGateway, blueprintID string) (usecase.EstimateSessionDetail, error) {
	sess, err := sessions.Get(ctx, blueprintID)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	msgs, err := sessions.Messages(ctx, blueprintID)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	return usecase.EstimateSessionDetail{Session: sess, Messages: msgs}, nil
}
