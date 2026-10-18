package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type photoDeleter struct {
	photos gateway.PhotoGateway
	authz  usecase.Authorizer
	guard  helper.WriteGuard
}

func NewPhotoDeleter(photos gateway.PhotoGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.PhotoDeleter {
	return &photoDeleter{photos: photos, authz: authz, guard: guard}
}

// Delete requires can_delete. It removes every tuple on the photo, then
// the row and its grants.
func (s *photoDeleter) Delete(ctx context.Context, actor, tenantID, id string) error {
	if _, err := loadPhoto(ctx, s.photos, tenantID, id); err != nil {
		return err
	}
	if err := s.authz.Require(ctx, actor, "can_delete", entity.ObjectTypePhoto, id); err != nil {
		return err
	}
	return s.guard.RevokeObject(ctx, entity.ObjectTypePhoto, id, func(ctx context.Context) error {
		return s.photos.Delete(ctx, id)
	})
}
