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

type photoGetter struct {
	photos gateway.PhotoGateway
	authz  helper.Authorizer
}

func NewPhotoGetter(photos gateway.PhotoGateway, authz helper.Authorizer) usecase.PhotoGetter {
	return &photoGetter{photos: photos, authz: authz}
}

// Get is 404 when the caller cannot view the photo.
func (s *photoGetter) Get(ctx context.Context, in input.PhotoGetter) (output.PhotoGetter, error) {
	p, err := loadPhoto(ctx, s.photos, in.TenantID, in.ID)
	if err != nil {
		return output.PhotoGetter{}, err
	}
	if err := s.authz.RequireVisible(ctx, in.Actor, "can_view", model.ObjectTypePhoto, in.ID); err != nil {
		return output.PhotoGetter{}, err
	}
	vs, err := withPerms(ctx, s.authz, in.Actor, model.ObjectTypePhoto, []model.Photo{p}, photoID)
	if err != nil {
		return output.PhotoGetter{}, err
	}
	return output.PhotoGetter{Visible: visibleOutput(vs[0], photoOutput)}, nil
}
