package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type albumVisibilitySetter struct {
	albums    gateway.AlbumGateway
	relations gateway.AlbumRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewAlbumVisibilitySetter(albums gateway.AlbumGateway, relations gateway.AlbumRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.AlbumVisibilitySetter {
	return &albumVisibilitySetter{albums: albums, relations: relations, authz: authz, guard: guard}
}

// SetVisibility makes an album private or public. Making it public grants
// external members access (a grant); making it private removes it (a revoke).
func (s *albumVisibilitySetter) SetVisibility(ctx context.Context, actor, tenantID, id string, isPrivate bool) (entity.Album, error) {
	a, err := loadAlbum(ctx, s.albums, tenantID, id)
	if err != nil {
		return entity.Album{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_change_visibility", entity.ObjectTypeAlbum, id); err != nil {
		return entity.Album{}, err
	}
	if a.IsPrivate == isPrivate {
		return a, nil
	}

	public := []entity.Tuple{s.relations.Public(tenantID, id)}
	makePrivate := func(ctx context.Context) error {
		_, err := s.albums.SetPrivate(ctx, id, true)
		return err
	}
	if isPrivate {
		err = s.guard.Revoke(ctx, public, public, makePrivate)
	} else {
		if _, err := s.albums.SetPrivate(ctx, id, false); err != nil {
			return entity.Album{}, err
		}
		err = s.guard.Grant(ctx, public, nil, makePrivate)
	}
	if err != nil {
		return entity.Album{}, err
	}
	return s.albums.Get(ctx, id)
}
