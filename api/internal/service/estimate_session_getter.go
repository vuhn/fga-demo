package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type estimateSessionGetter struct {
	blueprints gateway.BlueprintGateway
	sessions   gateway.EstimateSessionGateway
	estimates  gateway.BlueprintEstimateGateway
	authz      usecase.Authorizer
}

func NewEstimateSessionGetter(blueprints gateway.BlueprintGateway, sessions gateway.EstimateSessionGateway, estimates gateway.BlueprintEstimateGateway, authz usecase.Authorizer) usecase.EstimateSessionGetter {
	return &estimateSessionGetter{blueprints: blueprints, sessions: sessions, estimates: estimates, authz: authz}
}

// Get returns the session with its messages. The chat is can_edit; the Check fails as 404 so a viewer learns
// nothing about the session (they read the finished estimate instead).
func (s *estimateSessionGetter) Get(ctx context.Context, actor, tenantID, sessionID string) (usecase.EstimateSessionDetail, error) {
	sess, err := loadEstimateSession(ctx, s.sessions, tenantID, sessionID)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, sess.BlueprintID); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_edit", entity.ObjectTypeBlueprint, sess.BlueprintID); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	return getEstimateSession(ctx, s.sessions, s.estimates, tenantID, sessionID)
}

// getEstimateSession is the read after a Check (or after our own write).
func getEstimateSession(ctx context.Context, sessions gateway.EstimateSessionGateway, estimates gateway.BlueprintEstimateGateway, tenantID, id string) (usecase.EstimateSessionDetail, error) {
	sess, err := loadEstimateSession(ctx, sessions, tenantID, id)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	msgs, err := sessions.Messages(ctx, id)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	est, err := latestEstimate(ctx, estimates, sess)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	return usecase.EstimateSessionDetail{Session: sess, Messages: msgs, Estimate: est}, nil
}

type estimateSessionLister struct {
	blueprints gateway.BlueprintGateway
	sessions   gateway.EstimateSessionGateway
	authz      usecase.Authorizer
}

func NewEstimateSessionLister(blueprints gateway.BlueprintGateway, sessions gateway.EstimateSessionGateway, authz usecase.Authorizer) usecase.EstimateSessionLister {
	return &estimateSessionLister{blueprints: blueprints, sessions: sessions, authz: authz}
}

func (s *estimateSessionLister) List(ctx context.Context, actor, tenantID, blueprintID string) ([]entity.EstimateSession, error) {
	if _, err := loadBlueprint(ctx, s.blueprints, tenantID, blueprintID); err != nil {
		return nil, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_edit", entity.ObjectTypeBlueprint, blueprintID); err != nil {
		return nil, err
	}
	return s.sessions.ListByBlueprint(ctx, blueprintID)
}
