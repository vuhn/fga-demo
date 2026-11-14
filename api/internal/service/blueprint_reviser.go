package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintReviser struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
}

func NewBlueprintReviser(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintReviser {
	return &blueprintReviser{blueprints: blueprints, engine: engine, authz: authz}
}

// Revise requires can_edit and a current base_version. The engine turns the
// message into edits on the brief and rebuilds, keeping unchanged rooms in
// place; the result is a new version. Versions never touch OpenFGA.
func (s *blueprintReviser) Revise(ctx context.Context, actor, tenantID, id, message string, baseVersion int) (usecase.BlueprintDetail, error) {
	message = strings.TrimSpace(message)
	if n := len([]rune(message)); n < 1 || n > 2000 {
		return usecase.BlueprintDetail{}, entity.Invalid("message length must be 1 to 2000")
	}
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, actor, tenantID, id, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	history, err := chatHistory(ctx, s.blueprints, id)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	out, err := s.engine.Revise(ctx, cur.Brief, cur.Model, message, history)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	return appendBlueprintVersion(ctx, s.blueprints, b, outcomeVersion(id, entity.BlueprintVersionRevise, message, actor, out), baseVersion)
}
