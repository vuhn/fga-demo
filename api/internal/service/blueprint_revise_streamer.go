package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintReviseStreamer struct {
	blueprints gateway.BlueprintGateway
	agent      gateway.BlueprintAgent
	authz      usecase.Authorizer
}

func NewBlueprintReviseStreamer(blueprints gateway.BlueprintGateway, agent gateway.BlueprintAgent, authz usecase.Authorizer) usecase.BlueprintReviseStreamer {
	return &blueprintReviseStreamer{blueprints: blueprints, agent: agent, authz: authz}
}

// ReviseStream is Revise with live progress: the same can_edit Check and
// base_version test first, the same engine call, the same single write after
// it. Progress is passed through and never stored.
func (s *blueprintReviseStreamer) ReviseStream(ctx context.Context, actor, tenantID, id, message string, baseVersion int, onProgress func(entity.BlueprintProgress)) (usecase.BlueprintDetail, error) {
	message, b, cur, history, err := prepareRevision(ctx, s.blueprints, s.authz, actor, tenantID, id, message, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	imported := isImported(cur)
	var out entity.BlueprintOutcome
	if imported {
		out, err = s.agent.RevisePlanStream(ctx, cur.Model, message, history, onProgress)
	} else {
		out, err = s.agent.ReviseStream(ctx, cur.Brief, cur.Model, message, history, onProgress)
	}
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if imported {
		return appendBlueprintVersion(ctx, s.blueprints, b, planOutcomeVersion(id, entity.BlueprintVersionRevise, message, actor, out), baseVersion)
	}
	return appendBlueprintVersion(ctx, s.blueprints, b, outcomeVersion(id, entity.BlueprintVersionRevise, message, actor, out), baseVersion)
}
