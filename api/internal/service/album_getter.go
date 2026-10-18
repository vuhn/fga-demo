package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type albumGetter struct {
	albums gateway.AlbumGateway
	authz  usecase.Authorizer
}

func NewAlbumGetter(albums gateway.AlbumGateway, authz usecase.Authorizer) usecase.AlbumGetter {
	return &albumGetter{albums: albums, authz: authz}
}

// Get is 404 when the caller cannot view the album.
func (s *albumGetter) Get(ctx context.Context, actor, tenantID, id string) (entity.Visible[entity.Album], error) {
	a, err := loadAlbum(ctx, s.albums, tenantID, id)
	if err != nil {
		return entity.Visible[entity.Album]{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", entity.ObjectTypeAlbum, id); err != nil {
		return entity.Visible[entity.Album]{}, err
	}
	vs, err := withPerms(ctx, s.authz, actor, entity.ObjectTypeAlbum, []entity.Album{a}, albumID)
	if err != nil {
		return entity.Visible[entity.Album]{}, err
	}
	return vs[0], nil
}
