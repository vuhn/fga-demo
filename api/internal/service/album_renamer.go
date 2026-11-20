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

type albumRenamer struct {
	albums gateway.AlbumGateway
	authz  helper.Authorizer
}

func NewAlbumRenamer(albums gateway.AlbumGateway, authz helper.Authorizer) usecase.AlbumRenamer {
	return &albumRenamer{albums: albums, authz: authz}
}

// Rename requires can_manage: a userset editor can fill an album but cannot
// rename it.
func (s *albumRenamer) Rename(ctx context.Context, in input.AlbumRenamer) (output.AlbumRenamer, error) {
	if _, err := loadAlbum(ctx, s.albums, in.TenantID, in.ID); err != nil {
		return output.AlbumRenamer{}, err
	}
	name, err := model.CleanName("name", in.Name)
	if err != nil {
		return output.AlbumRenamer{}, err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_manage", model.ObjectTypeAlbum, in.ID); err != nil {
		return output.AlbumRenamer{}, err
	}
	got, err := s.albums.Rename(ctx, in.ID, name)
	return output.AlbumRenamer{Album: albumOutput(got)}, err
}
