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

type albumVisibilitySetter struct {
	albums    gateway.AlbumGateway
	relations gateway.AlbumRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewAlbumVisibilitySetter(albums gateway.AlbumGateway, relations gateway.AlbumRelationGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.AlbumVisibilitySetter {
	return &albumVisibilitySetter{albums: albums, relations: relations, authz: authz, guard: guard}
}

// SetVisibility makes an album private or public. Making it public grants
// external members access (a grant); making it private removes it (a revoke).
func (s *albumVisibilitySetter) SetVisibility(ctx context.Context, in input.AlbumVisibilitySetter) (output.AlbumVisibilitySetter, error) {
	a, err := loadAlbum(ctx, s.albums, in.TenantID, in.ID)
	if err != nil {
		return output.AlbumVisibilitySetter{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_change_visibility", model.ObjectTypeAlbum, in.ID); err != nil {
		return output.AlbumVisibilitySetter{}, err
	}
	if a.IsPrivate == in.IsPrivate {
		return output.AlbumVisibilitySetter{Album: albumOutput(a)}, nil
	}

	public := []model.Tuple{s.relations.Public(in.TenantID, in.ID)}
	makePrivate := func(ctx context.Context) error {
		_, err := s.albums.SetPrivate(ctx, in.ID, true)
		return err
	}
	if in.IsPrivate {
		err = s.guard.Revoke(ctx, public, public, makePrivate)
	} else {
		if _, err := s.albums.SetPrivate(ctx, in.ID, false); err != nil {
			return output.AlbumVisibilitySetter{}, err
		}
		err = s.guard.Grant(ctx, public, nil, makePrivate)
	}
	if err != nil {
		return output.AlbumVisibilitySetter{}, err
	}
	got, err := s.albums.Get(ctx, in.ID)
	return output.AlbumVisibilitySetter{Album: albumOutput(got)}, err
}
