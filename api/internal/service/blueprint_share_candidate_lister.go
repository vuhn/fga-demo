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

type blueprintShareCandidateLister struct {
	blueprints gateway.BlueprintGateway
	members    gateway.MembershipGateway
	authz      helper.Authorizer
}

func NewBlueprintShareCandidateLister(blueprints gateway.BlueprintGateway, members gateway.MembershipGateway, authz helper.Authorizer) usecase.BlueprintShareCandidateLister {
	return &blueprintShareCandidateLister{blueprints: blueprints, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the blueprint's tenant. It
// needs can_share on the blueprint, not member management, so owners can share
// without seeing the full roster.
func (s *blueprintShareCandidateLister) ListShareCandidates(ctx context.Context, in input.BlueprintShareCandidateLister) (output.BlueprintShareCandidateLister, error) {
	if err := requireBlueprintShare(ctx, s.blueprints, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.BlueprintShareCandidateLister{}, err
	}
	got, err := s.members.List(ctx, in.TenantID, model.AffiliationExternal)
	return output.BlueprintShareCandidateLister{Items: mapOutput(got, membershipOutput)}, err
}
