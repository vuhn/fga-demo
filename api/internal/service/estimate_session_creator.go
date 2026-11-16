package service

import (
	"context"
	"encoding/json"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type estimateSessionCreator struct {
	blueprints gateway.BlueprintGateway
	sessions   gateway.EstimateSessionGateway
	authz      usecase.Authorizer
}

func NewEstimateSessionCreator(blueprints gateway.BlueprintGateway, sessions gateway.EstimateSessionGateway, authz usecase.Authorizer) usecase.EstimateSessionCreator {
	return &estimateSessionCreator{blueprints: blueprints, sessions: sessions, authz: authz}
}

// Create requires can_edit on the blueprint (a session spends LLM and search calls). A blueprint has one
// session: starting one resets the old (its chat and answers), while finished estimates stay. The session is
// pinned to the blueprint's current version; a later plan edit leaves it stale rather than changing its numbers.
func (s *estimateSessionCreator) Create(ctx context.Context, actor, tenantID, blueprintID string) (usecase.EstimateSessionDetail, error) {
	b, err := loadBlueprint(ctx, s.blueprints, tenantID, blueprintID)
	if err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", entity.ObjectTypeBlueprint, blueprintID); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	sess := entity.EstimateSession{BlueprintID: b.ID, BlueprintVersion: b.CurrentVersion, State: json.RawMessage(`{}`), CreatedBy: actor}
	if err := s.sessions.Replace(ctx, sess); err != nil {
		return usecase.EstimateSessionDetail{}, err
	}
	return readEstimateSession(ctx, s.sessions, b.ID)
}
