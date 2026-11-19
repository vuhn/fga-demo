package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type photoGetter struct {
	photos gateway.PhotoGateway
	authz  usecase.Authorizer
}

func NewPhotoGetter(photos gateway.PhotoGateway, authz usecase.Authorizer) usecase.PhotoGetter {
	return &photoGetter{photos: photos, authz: authz}
}

// Get is 404 when the caller cannot view the photo.
func (s *photoGetter) Get(ctx context.Context, actor, tenantID, id string) (model.Visible[model.Photo], error) {
	p, err := loadPhoto(ctx, s.photos, tenantID, id)
	if err != nil {
		return model.Visible[model.Photo]{}, err
	}
	if err := s.authz.RequireVisible(ctx, actor, "can_view", model.ObjectTypePhoto, id); err != nil {
		return model.Visible[model.Photo]{}, err
	}
	vs, err := withPerms(ctx, s.authz, actor, model.ObjectTypePhoto, []model.Photo{p}, photoID)
	if err != nil {
		return model.Visible[model.Photo]{}, err
	}
	return vs[0], nil
}
