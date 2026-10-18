package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
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
func (s *photoLister) List(ctx context.Context, actor, tenantID, albumID string) ([]entity.Visible[entity.Photo], error) {
	if albumID != "" {
		if err := entity.RequireUUID("album_id", albumID); err != nil {
			return nil, err
		}
	}
	ids, err := s.authz.ListObjectIDs(ctx, actor, "can_view", entity.ObjectTypePhoto)
	if err != nil || len(ids) == 0 {
		return []entity.Visible[entity.Photo]{}, err
	}
	photos, err := s.photos.ListByIDs(ctx, tenantID, ids, albumID)
	if err != nil {
		return nil, err
	}
	return withPerms(ctx, s.authz, actor, entity.ObjectTypePhoto, photos, photoID)
}
