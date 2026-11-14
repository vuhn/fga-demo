package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintReviewer struct {
	blueprints gateway.BlueprintGateway
	engine     gateway.BlueprintEngine
	authz      usecase.Authorizer
}

func NewBlueprintReviewer(blueprints gateway.BlueprintGateway, engine gateway.BlueprintEngine, authz usecase.Authorizer) usecase.BlueprintReviewer {
	return &blueprintReviewer{blueprints: blueprints, engine: engine, authz: authz}
}

// Review requires can_edit and a current base_version, like a chat revision.
// The engine's reviewer proposes fixes for the failed checks and keeps only
// rounds that lower the issue score; its round notes become the new
// version's notes. A review that changes nothing is a 400 carrying those
// notes, and nothing is stored.
func (s *blueprintReviewer) Review(ctx context.Context, actor, tenantID, id string, baseVersion int) (usecase.BlueprintDetail, error) {
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, actor, tenantID, id, baseVersion)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	history, err := chatHistory(ctx, s.blueprints, id)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	out, err := s.engine.Review(ctx, cur.Brief, cur.Model, history)
	if err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if err := requireBuilt(out); err != nil {
		return usecase.BlueprintDetail{}, err
	}
	if sameJSON(out.Brief, cur.Brief) {
		msg := "nothing to fix"
		if len(out.Notes) > 0 {
			msg = "no fix found: " + strings.Join(out.Notes, "; ")
		}
		return usecase.BlueprintDetail{}, entity.Invalid("%s", msg)
	}
	return appendBlueprintVersion(ctx, s.blueprints, b, outcomeVersion(id, entity.BlueprintVersionReview, "", actor, out), baseVersion)
}
