package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type albumDeleter struct {
	albums gateway.AlbumGateway
	authz  helper.Authorizer
	guard  helper.WriteGuard
}

func NewAlbumDeleter(albums gateway.AlbumGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.AlbumDeleter {
	return &albumDeleter{albums: albums, authz: authz, guard: guard}
}

// Delete requires can_delete and an empty album. It removes every tuple on
// the album, then the row and its grants.
func (s *albumDeleter) Delete(ctx context.Context, in input.AlbumDeleter) error {
	if _, err := loadAlbum(ctx, s.albums, in.TenantID, in.ID); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_delete", model.ObjectTypeAlbum, in.ID); err != nil {
		return err
	}
	has, err := s.albums.HasChildren(ctx, in.ID)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("%w: album still has children", model.ErrConflict)
	}
	return s.guard.RevokeObject(ctx, model.ObjectTypeAlbum, in.ID, func(ctx context.Context) error {
		return s.albums.Delete(ctx, in.ID)
	})
}
