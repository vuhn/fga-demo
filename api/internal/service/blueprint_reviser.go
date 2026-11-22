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

type blueprintReviser struct {
	blueprints gateway.BlueprintGateway
	agent      gateway.BlueprintAgent
	authz      helper.Authorizer
}

func NewBlueprintReviser(blueprints gateway.BlueprintGateway, agent gateway.BlueprintAgent, authz helper.Authorizer) usecase.BlueprintReviser {
	return &blueprintReviser{blueprints: blueprints, agent: agent, authz: authz}
}

// Revise requires can_edit and a current base_version. The engine turns the
// message into edits on the brief and rebuilds, keeping unchanged rooms in
// place; the result is a new version. Versions never touch OpenFGA.
func (s *blueprintReviser) Revise(ctx context.Context, in input.BlueprintReviser) (output.BlueprintReviser, error) {
	message, _, cur, history, err := prepareRevision(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID, in.Message, in.BaseVersion)
	if err != nil {
		return output.BlueprintReviser{}, err
	}
	if isImported(cur) { // no brief: the engine edits the model as drawn
		out, err := s.agent.RevisePlan(ctx, model.RevisePlanParams{Plan: cur.Model, Message: message, History: history})
		if err != nil {
			return output.BlueprintReviser{}, err
		}
		if err := requireBuilt(out); err != nil {
			return output.BlueprintReviser{}, err
		}
		got, err := appendBlueprintVersion(ctx, s.blueprints, planOutcomeVersion(in.ID, model.BlueprintVersionRevise, message, in.Actor, out), in.BaseVersion)
		return output.BlueprintReviser{BlueprintDetail: blueprintDetailOutput(got)}, err
	}
	out, err := s.agent.Revise(ctx, model.ReviseBlueprintParams{Brief: cur.Brief, Previous: cur.Model, Message: message, History: history})
	if err != nil {
		return output.BlueprintReviser{}, err
	}
	if err := requireBuilt(out); err != nil {
		return output.BlueprintReviser{}, err
	}
	got, err := appendBlueprintVersion(ctx, s.blueprints, outcomeVersion(in.ID, model.BlueprintVersionRevise, message, in.Actor, out), in.BaseVersion)
	return output.BlueprintReviser{BlueprintDetail: blueprintDetailOutput(got)}, err
}
