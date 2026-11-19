package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type photoLister struct {
	photos gateway.PhotoGateway
	authz  usecase.Authorizer
}

func NewPhotoLister(photos gateway.PhotoGateway, authz usecase.Authorizer) usecase.PhotoLister {
	return &photoLister{photos: photos, authz: authz}
}

// List returns every visible photo in the tenant, including photos
// whose album the actor cannot see ("Shared with me").
func (s *photoLister) List(ctx context.Context, actor, tenantID, albumID, projectID string) ([]model.Visible[model.Photo], error) {
	if albumID != "" {
		if err := model.RequireUUID("album_id", albumID); err != nil {
			return nil, err
		}
	}
	if projectID != "" {
		if err := model.RequireUUID("project_id", projectID); err != nil {
			return nil, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_view", model.ObjectTypePhoto)
	if err != nil || len(ids) == 0 {
		return []model.Visible[model.Photo]{}, err
	}
	photos, err := s.photos.ListByIDs(ctx, tenantID, ids, albumID, projectID)
	if err != nil {
		return nil, err
	}
	return withPerms(ctx, s.authz, actor, model.ObjectTypePhoto, photos, photoID)
}
