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

type blueprintReviseStreamer struct {
	blueprints gateway.BlueprintGateway
	agent      gateway.BlueprintAgent
	authz      helper.Authorizer
}

func NewBlueprintReviseStreamer(blueprints gateway.BlueprintGateway, agent gateway.BlueprintAgent, authz helper.Authorizer) usecase.BlueprintReviseStreamer {
	return &blueprintReviseStreamer{blueprints: blueprints, agent: agent, authz: authz}
}

// ReviseStream is Revise with live progress: the same can_edit Check and
// base_version test first, the same engine call, the same single write after
// it. Progress is passed through and never stored.
func (s *blueprintReviseStreamer) ReviseStream(ctx context.Context, in input.BlueprintReviseStreamer, onProgress func(model.BlueprintProgress)) (output.BlueprintReviseStreamer, error) {
	message, _, cur, history, err := prepareRevision(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID, in.Message, in.BaseVersion)
	if err != nil {
		return output.BlueprintReviseStreamer{}, err
	}
	imported := isImported(cur)
	var out model.BlueprintOutcome
	if imported {
		out, err = s.agent.RevisePlanStream(ctx, model.RevisePlanParams{Plan: cur.Model, Message: message, History: history}, onProgress)
	} else {
		out, err = s.agent.ReviseStream(ctx, model.ReviseBlueprintParams{Brief: cur.Brief, Previous: cur.Model, Message: message, History: history}, onProgress)
	}
	if err != nil {
		return output.BlueprintReviseStreamer{}, err
	}
	if err := requireBuilt(out); err != nil {
		return output.BlueprintReviseStreamer{}, err
	}
	if imported {
		got, err := appendBlueprintVersion(ctx, s.blueprints, planOutcomeVersion(in.ID, model.BlueprintVersionRevise, message, in.Actor, out), in.BaseVersion)
		return output.BlueprintReviseStreamer{BlueprintDetail: blueprintDetailOutput(got)}, err
	}
	got, err := appendBlueprintVersion(ctx, s.blueprints, outcomeVersion(in.ID, model.BlueprintVersionRevise, message, in.Actor, out), in.BaseVersion)
	return output.BlueprintReviseStreamer{BlueprintDetail: blueprintDetailOutput(got)}, err
}
