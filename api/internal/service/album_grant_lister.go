package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type albumGrantLister struct {
	albums gateway.AlbumGateway
	grants gateway.AlbumGrantGateway
	authz  usecase.Authorizer
}

func NewAlbumGrantLister(albums gateway.AlbumGateway, grants gateway.AlbumGrantGateway, authz usecase.Authorizer) usecase.AlbumGrantLister {
	return &albumGrantLister{albums: albums, grants: grants, authz: authz}
}

func (s *albumGrantLister) ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error) {
	if err := requireAlbumShare(ctx, s.albums, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.grants.List(ctx, id)
}
