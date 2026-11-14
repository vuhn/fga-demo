package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintReviseStreamer struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
}

func NewBlueprintReviseStreamer(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintReviseStreamer {
	return &blueprintReviseStreamer{blueprints: blueprints, engine: engine, authz: authz}
}

// ReviseStream is Revise with live progress: the same can_edit Check and
// base_version test first, the same engine call, the same single write after
// it. Progress is passed through and never stored.
func (s *blueprintReviseStreamer) ReviseStream(ctx context.Context, actor, tenantID, id, message string, baseVersion int, onProgress func(entity.BlueprintProgress)) (usecase.BlueprintDetail, error) {
	message, b, cur, history, err := prepareRevision(ctx, s.blueprints, s.authz, actor, tenantID, id, message, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	out, err := s.engine.ReviseStream(ctx, cur.Brief, cur.Model, message, history, onProgress)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	return appendBlueprintVersion(ctx, s.blueprints, b, outcomeVersion(id, entity.BlueprintVersionRevise, message, actor, out), baseVersion)
}
