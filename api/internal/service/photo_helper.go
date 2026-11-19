package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

// loadPhoto returns the photo when it exists in the header tenant.
func loadPhoto(ctx context.Context, photos gateway.PhotoGateway, tenantID, id string) (model.Photo, error) {
	if err := model.RequireUUID("id", id); err != nil {
		return model.Photo{}, err
	}
	p, err := photos.Get(ctx, id)
	if err != nil {
		return model.Photo{}, err
	}
	if p.TenantID != tenantID {
		return model.Photo{}, model.ErrNotFound
	}
	return p, nil
}

// requirePhotoShare loads the photo in the tenant and requires can_share.
func requirePhotoShare(ctx context.Context, photos gateway.PhotoGateway, authz usecase.Authorizer, actor, tenantID, id string) error {
	if _, err := loadPhoto(ctx, photos, tenantID, id); err != nil {
		return err
	}
	return authz.Require(ctx, actor, "can_share", model.ObjectTypePhoto, id)
}

func photoID(p model.Photo) string { return p.ID }
