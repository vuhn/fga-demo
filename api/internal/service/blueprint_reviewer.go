package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type blueprintReviewer struct {
	blueprints gateway.BlueprintGateway
	agent      gateway.BlueprintAgent
	authz      helper.Authorizer
}

func NewBlueprintReviewer(blueprints gateway.BlueprintGateway, agent gateway.BlueprintAgent, authz helper.Authorizer) usecase.BlueprintReviewer {
	return &blueprintReviewer{blueprints: blueprints, agent: agent, authz: authz}
}

// Review requires can_edit and a current base_version, like a chat revision.
// The engine's reviewer proposes fixes for the failed checks and keeps only
// rounds that lower the issue score; its round notes become the new
// version's notes. A review that changes nothing is a 400 carrying those
// notes, and nothing is stored.
func (s *blueprintReviewer) Review(ctx context.Context, in input.BlueprintReviewer) (output.BlueprintReviewer, error) {
	b, cur, err := loadBlueprintForEdit(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID, in.BaseVersion)
	if err != nil {
		return output.BlueprintReviewer{}, err
	}
	if err := requireBrief(cur); err != nil {
		return output.BlueprintReviewer{}, err
	}
	history, err := chatHistory(ctx, s.blueprints, in.ID)
	if err != nil {
		return output.BlueprintReviewer{}, err
	}
	out, err := s.agent.Review(ctx, cur.Brief, cur.Model, history)
	if err != nil {
		return output.BlueprintReviewer{}, err
	}
	if err := requireBuilt(out); err != nil {
		return output.BlueprintReviewer{}, err
	}
	if sameJSON(out.Brief, cur.Brief) {
		msg := "nothing to fix"
		if len(out.Notes) > 0 {
			msg = "no fix found: " + strings.Join(out.Notes, "; ")
		}
		return output.BlueprintReviewer{}, model.Invalid("%s", msg)
	}
	got, err := appendBlueprintVersion(ctx, s.blueprints, b, outcomeVersion(in.ID, model.BlueprintVersionReview, "", in.Actor, out), in.BaseVersion)
	return output.BlueprintReviewer{BlueprintDetail: blueprintDetailOutput(got)}, err
}
