package service

import (
	"context"
	"strings"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintDrafter struct {
	projects gateway.ProjectGateway
	agent    gateway.BlueprintAgent
	authz    usecase.Authorizer
}

func NewBlueprintDrafter(projects gateway.ProjectGateway, agent gateway.BlueprintAgent, authz usecase.Authorizer) usecase.BlueprintDrafter {
	return &blueprintDrafter{projects: projects, agent: agent, authz: authz}
}

// Draft requires can_create_blueprint on the project, then asks the engine
// for a brief. Nothing is stored.
func (s *blueprintDrafter) Draft(ctx context.Context, actor, tenantID, projectID, prompt string) (model.BlueprintDraft, error) {
	prompt = strings.TrimSpace(prompt)
	if n := len([]rune(prompt)); n < 1 || n > 4000 {
		return model.BlueprintDraft{}, model.Invalid("prompt length must be 1 to 4000")
	}
	if err := requireBlueprintCreate(ctx, s.projects, s.authz, actor, tenantID, projectID); err != nil {
		return model.BlueprintDraft{}, err
	}
	return s.agent.BriefFromText(ctx, prompt)
}
