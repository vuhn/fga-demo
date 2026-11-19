package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintReviser struct {
	blueprints gateway.BlueprintGateway
	agent      gateway.BlueprintAgent
	authz      usecase.Authorizer
}

func NewBlueprintReviser(blueprints gateway.BlueprintGateway, agent gateway.BlueprintAgent, authz usecase.Authorizer) usecase.BlueprintReviser {
	return &blueprintReviser{blueprints: blueprints, agent: agent, authz: authz}
}

// Revise requires can_edit and a current base_version. The engine turns the
// message into edits on the brief and rebuilds, keeping unchanged rooms in
// place; the result is a new version. Versions never touch OpenFGA.
func (s *blueprintReviser) Revise(ctx context.Context, actor, tenantID, id, message string, baseVersion int) (usecase.BlueprintDetail, error) {
	message, b, cur, history, err := prepareRevision(ctx, s.blueprints, s.authz, actor, tenantID, id, message, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if isImported(cur) { // no brief: the engine edits the model as drawn
		out, err := s.agent.RevisePlan(ctx, cur.Model, message, history)
		if err != nil {
			return usecase.BlueprintDetail{}, err
		}
		if err := requireBuilt(out); err != nil {
			return usecase.BlueprintDetail{}, err
		}
		return appendBlueprintVersion(ctx, s.blueprints, b, planOutcomeVersion(id, entity.BlueprintVersionRevise, message, actor, out), baseVersion)
	}
	out, err := s.agent.Revise(ctx, cur.Brief, cur.Model, message, history)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	return appendBlueprintVersion(ctx, s.blueprints, b, outcomeVersion(id, entity.BlueprintVersionRevise, message, actor, out), baseVersion)
}
