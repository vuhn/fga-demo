package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type albumLister struct {
	albums gateway.AlbumGateway
	authz  usecase.Authorizer
}

func NewAlbumLister(albums gateway.AlbumGateway, authz usecase.Authorizer) usecase.AlbumLister {
	return &albumLister{albums: albums, authz: authz}
}

// List is ListObjects can_view, then a Postgres load of those ids in the
// tenant, with the caller's permissions on each.
func (s *albumLister) List(ctx context.Context, actor, tenantID string) ([]entity.Visible[entity.Album], error) {
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_view", entity.ObjectTypeAlbum)
	if err != nil || len(ids) == 0 {
		return []entity.Visible[entity.Album]{}, err
	}
	albums, err := s.albums.ListByIDs(ctx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return withPerms(ctx, s.authz, actor, entity.ObjectTypeAlbum, albums, albumID)
}
