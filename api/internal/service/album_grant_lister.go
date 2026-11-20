package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type albumGrantLister struct {
	albums gateway.AlbumGateway
	grants gateway.AlbumGrantGateway
	authz  helper.Authorizer
}

func NewAlbumGrantLister(albums gateway.AlbumGateway, grants gateway.AlbumGrantGateway, authz helper.Authorizer) usecase.AlbumGrantLister {
	return &albumGrantLister{albums: albums, grants: grants, authz: authz}
}

func (s *albumGrantLister) ListGrants(ctx context.Context, in input.AlbumGrantLister) (output.AlbumGrantLister, error) {
	if err := requireAlbumShare(ctx, s.albums, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.AlbumGrantLister{}, err
	}
	got, err := s.grants.List(ctx, in.ID)
	return output.AlbumGrantLister{Items: mapOutput(got, grantOutput)}, err
}
