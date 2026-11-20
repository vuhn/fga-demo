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

type photoShareCandidateLister struct {
	photos  gateway.PhotoGateway
	members gateway.MembershipGateway
	authz   helper.Authorizer
}

func NewPhotoShareCandidateLister(photos gateway.PhotoGateway, members gateway.MembershipGateway, authz helper.Authorizer) usecase.PhotoShareCandidateLister {
	return &photoShareCandidateLister{photos: photos, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the photo's tenant. It
// needs can_share on the photo, not member management, so owners can share
// without seeing the full roster.
func (s *photoShareCandidateLister) ListShareCandidates(ctx context.Context, in input.PhotoShareCandidateLister) (output.PhotoShareCandidateLister, error) {
	if err := requirePhotoShare(ctx, s.photos, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.PhotoShareCandidateLister{}, err
	}
	got, err := s.members.List(ctx, in.TenantID, model.AffiliationExternal)
	return output.PhotoShareCandidateLister{Items: mapOutput(got, membershipOutput)}, err
}
