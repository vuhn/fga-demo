package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type photoRenamer struct {
	photos gateway.PhotoGateway
	authz  usecase.Authorizer
}

func NewPhotoRenamer(photos gateway.PhotoGateway, authz usecase.Authorizer) usecase.PhotoRenamer {
	return &photoRenamer{photos: photos, authz: authz}
}

// Rename requires can_edit.
func (s *photoRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (entity.Photo, error) {
	if _, err := loadPhoto(ctx, s.photos, tenantID, id); err != nil {
		return entity.Photo{}, err
	}
	name, err := entity.CleanName("name", name)
	if err != nil {
		return entity.Photo{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", entity.ObjectTypePhoto, id); err != nil {
		return entity.Photo{}, err
	}
	return s.photos.Rename(ctx, id, name)
}
