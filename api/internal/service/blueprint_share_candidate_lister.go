package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type blueprintShareCandidateLister struct {
	blueprints gateway.BlueprintGateway
	members    gateway.MembershipGateway
	authz      usecase.Authorizer
}

func NewBlueprintShareCandidateLister(blueprints gateway.BlueprintGateway, members gateway.MembershipGateway, authz usecase.Authorizer) usecase.BlueprintShareCandidateLister {
	return &blueprintShareCandidateLister{blueprints: blueprints, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the blueprint's tenant. It
// needs can_share on the blueprint, not member management, so owners can share
// without seeing the full roster.
func (s *blueprintShareCandidateLister) ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error) {
	if err := requireBlueprintShare(ctx, s.blueprints, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.members.List(ctx, tenantID, entity.AffiliationExternal)
}
