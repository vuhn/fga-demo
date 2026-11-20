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

type blueprintDrafter struct {
	projects gateway.ProjectGateway
	agent    gateway.BlueprintAgent
	authz    helper.Authorizer
}

func NewBlueprintDrafter(projects gateway.ProjectGateway, agent gateway.BlueprintAgent, authz helper.Authorizer) usecase.BlueprintDrafter {
	return &blueprintDrafter{projects: projects, agent: agent, authz: authz}
}

// Draft requires can_create_blueprint on the project, then asks the engine
// for a brief. Nothing is stored.
func (s *blueprintDrafter) Draft(ctx context.Context, in input.BlueprintDrafter) (output.BlueprintDrafter, error) {
	in.Prompt = strings.TrimSpace(in.Prompt)
	if n := len([]rune(in.Prompt)); n < 1 || n > 4000 {
		return output.BlueprintDrafter{}, model.Invalid("prompt length must be 1 to 4000")
	}
	if err := requireBlueprintCreate(ctx, s.projects, s.authz, in.Actor, in.TenantID, in.ProjectID); err != nil {
		return output.BlueprintDrafter{}, err
	}
	got, err := s.agent.BriefFromText(ctx, in.Prompt)
	return output.BlueprintDrafter{BlueprintDraft: blueprintDraftOutput(got)}, err
}
