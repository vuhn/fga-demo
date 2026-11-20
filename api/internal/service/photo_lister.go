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

type photoLister struct {
	photos gateway.PhotoGateway
	authz  helper.Authorizer
}

func NewPhotoLister(photos gateway.PhotoGateway, authz helper.Authorizer) usecase.PhotoLister {
	return &photoLister{photos: photos, authz: authz}
}

// List returns every visible photo in the tenant, including photos
// whose album the actor cannot see ("Shared with me").
func (s *photoLister) List(ctx context.Context, in input.PhotoLister) (output.PhotoLister, error) {
	if in.AlbumID != "" {
		if err := model.RequireUUID("album_id", in.AlbumID); err != nil {
			return output.PhotoLister{}, err
		}
	}
	if in.ProjectID != "" {
		if err := model.RequireUUID("project_id", in.ProjectID); err != nil {
			return output.PhotoLister{}, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, in.Actor, "can_view", model.ObjectTypePhoto)
	if err != nil || len(ids) == 0 {
		return output.PhotoLister{}, err
	}
	photos, err := s.photos.ListByIDs(ctx, model.ListPhotosParams{TenantID: in.TenantID, IDs: ids, AlbumID: in.AlbumID, ProjectID: in.ProjectID})
	if err != nil {
		return output.PhotoLister{}, err
	}
	got, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypePhoto, photos, photoID)
	return output.PhotoLister{Items: mapVisibleOutput(got, photoOutput)}, err
}
