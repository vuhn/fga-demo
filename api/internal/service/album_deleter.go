package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type albumDeleter struct {
	albums gateway.AlbumGateway
	authz  usecase.Authorizer
	guard  helper.WriteGuard
}

func NewAlbumDeleter(albums gateway.AlbumGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.AlbumDeleter {
	return &albumDeleter{albums: albums, authz: authz, guard: guard}
}

// Delete requires can_delete and an empty album. It removes every tuple on
// the album, then the row and its grants.
func (s *albumDeleter) Delete(ctx context.Context, actor, tenantID, id string) error {
	if _, err := loadAlbum(ctx, s.albums, tenantID, id); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, actor, "can_delete", entity.ObjectTypeAlbum, id); err != nil {
		return err
	}
	has, err := s.albums.HasChildren(ctx, id)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("%w: album still has children", entity.ErrConflict)
	}
	return s.guard.RevokeObject(ctx, entity.ObjectTypeAlbum, id, func(ctx context.Context) error {
		return s.albums.Delete(ctx, id)
	})
}
