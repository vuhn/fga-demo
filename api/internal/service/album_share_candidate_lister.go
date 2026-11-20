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

type albumShareCandidateLister struct {
	albums  gateway.AlbumGateway
	members gateway.MembershipGateway
	authz   helper.Authorizer
}

func NewAlbumShareCandidateLister(albums gateway.AlbumGateway, members gateway.MembershipGateway, authz helper.Authorizer) usecase.AlbumShareCandidateLister {
	return &albumShareCandidateLister{albums: albums, members: members, authz: authz}
}

// ListShareCandidates lists the external members of the album's tenant. It
// needs can_share on the album, not member management, so owners can share
// without seeing the full roster.
func (s *albumShareCandidateLister) ListShareCandidates(ctx context.Context, in input.AlbumShareCandidateLister) (output.AlbumShareCandidateLister, error) {
	if err := requireAlbumShare(ctx, s.albums, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.AlbumShareCandidateLister{}, err
	}
	got, err := s.members.List(ctx, in.TenantID, model.AffiliationExternal)
	return output.AlbumShareCandidateLister{Items: mapOutput(got, membershipOutput)}, err
}
