package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
)

type photoDeleter struct {
	photos gateway.PhotoGateway
	authz  helper.Authorizer
	guard  helper.WriteGuard
}

func NewPhotoDeleter(photos gateway.PhotoGateway, authz helper.Authorizer, guard helper.WriteGuard) usecase.PhotoDeleter {
	return &photoDeleter{photos: photos, authz: authz, guard: guard}
}

// Delete requires can_delete. It removes every tuple on the photo, then
// the row and its grants.
func (s *photoDeleter) Delete(ctx context.Context, in input.PhotoDeleter) error {
	if _, err := loadPhoto(ctx, s.photos, in.TenantID, in.ID); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, in.Actor, "can_delete", model.ObjectTypePhoto, in.ID); err != nil {
		return err
	}
	return s.guard.RevokeObject(ctx, model.ObjectTypePhoto, in.ID, func(ctx context.Context) error {
		return s.photos.Delete(ctx, in.ID)
	})
}
