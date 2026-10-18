package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type albumShareCandidateLister struct {
	albums  gateway.AlbumGateway
	members gateway.MembershipGateway
	authz   usecase.Authorizer
}

func NewAlbumShareCandidateLister(albums gateway.AlbumGateway, members gateway.MembershipGateway, authz usecase.Authorizer) usecase.AlbumShareCandidateLister {
	return &albumShareCandidateLister{albums: albums, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the album's tenant. It
// needs can_share on the album, not member management, so owners can share
// without seeing the full roster.
func (s *albumShareCandidateLister) ListShareCandidates(ctx context.Context, actor, tenantID, id string) ([]entity.Membership, error) {
	if err := requireAlbumShare(ctx, s.albums, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.members.List(ctx, tenantID, entity.AffiliationExternal)
}
