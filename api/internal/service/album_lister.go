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

type albumLister struct {
	albums gateway.AlbumGateway
	authz  helper.Authorizer
}

func NewAlbumLister(albums gateway.AlbumGateway, authz helper.Authorizer) usecase.AlbumLister {
	return &albumLister{albums: albums, authz: authz}
}

// List is ListObjects can_view, then a Postgres load of those ids in the
// tenant, with the caller's permissions on each.
func (s *albumLister) List(ctx context.Context, in input.AlbumLister) (output.AlbumLister, error) {
	if in.ProjectID != "" {
		if err := model.RequireUUID("project_id", in.ProjectID); err != nil {
			return output.AlbumLister{}, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, in.Actor, "can_view", model.ObjectTypeAlbum)
	if err != nil || len(ids) == 0 {
		return output.AlbumLister{}, err
	}
	albums, err := s.albums.ListByIDs(ctx, in.TenantID, ids, in.ProjectID)
	if err != nil {
		return output.AlbumLister{}, err
	}
	got, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypeAlbum, albums, albumID)
	return output.AlbumLister{Items: mapVisibleOutput(got, albumOutput)}, err
}
