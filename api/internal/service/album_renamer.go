package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type albumRenamer struct {
	albums gateway.AlbumGateway
	authz  usecase.Authorizer
}

func NewAlbumRenamer(albums gateway.AlbumGateway, authz usecase.Authorizer) usecase.AlbumRenamer {
	return &albumRenamer{albums: albums, authz: authz}
}

// Rename requires can_manage: a userset editor can fill an album but cannot
// rename it.
func (s *albumRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Album, error) {
	if _, err := loadAlbum(ctx, s.albums, tenantID, id); err != nil {
		return entity.Album{}, err
	}
	name, err := entity.CleanName("name", name)
	if err != nil {
		return entity.Album{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_manage", entity.ObjectTypeAlbum, id); err != nil {
		return entity.Album{}, err
	}
	return s.albums.Rename(ctx, id, name)
}
