package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type photoShareCandidateLister struct {
	photos  gateway.PhotoGateway
	members gateway.MembershipGateway
	authz   usecase.Authorizer
}

func NewPhotoShareCandidateLister(photos gateway.PhotoGateway, members gateway.MembershipGateway, authz usecase.Authorizer) usecase.PhotoShareCandidateLister {
	return &photoShareCandidateLister{photos: photos, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the photo's tenant. It
// needs can_share on the photo, not member management, so owners can share
// without seeing the full roster.
func (s *photoShareCandidateLister) ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error) {
	if err := requirePhotoShare(ctx, s.photos, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.members.List(ctx, tenantID, entity.AffiliationExternal)
}
