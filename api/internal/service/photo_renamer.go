package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
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
func (s *photoRenamer) Rename(ctx context.Context, actor, tenantID, id, name string) (model.Photo, error) {
	if _, err := loadPhoto(ctx, s.photos, tenantID, id); err != nil {
		return model.Photo{}, err
	}
	name, err := model.CleanName("name", name)
	if err != nil {
		return model.Photo{}, err
	}
	if err := s.authz.Require(ctx, actor, "can_edit", model.ObjectTypePhoto, id); err != nil {
		return model.Photo{}, err
	}
	return s.photos.Rename(ctx, id, name)
}
