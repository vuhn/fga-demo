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

type albumGetter struct {
	albums gateway.AlbumGateway
	authz  helper.Authorizer
}

func NewAlbumGetter(albums gateway.AlbumGateway, authz helper.Authorizer) usecase.AlbumGetter {
	return &albumGetter{albums: albums, authz: authz}
}

// Get is 404 when the caller cannot view the album.
func (s *albumGetter) Get(ctx context.Context, in input.AlbumGetter) (output.AlbumGetter, error) {
	a, err := loadAlbum(ctx, s.albums, in.TenantID, in.ID)
	if err != nil {
		return output.AlbumGetter{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypeAlbum, in.ID); err != nil {
		return output.AlbumGetter{}, err
	}
	vs, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeAlbum, []model.Album{a}, albumID)
	if err != nil {
		return output.AlbumGetter{}, err
	}
	return output.AlbumGetter{Visible: visibleOutput(vs[0], albumOutput)}, nil
}
