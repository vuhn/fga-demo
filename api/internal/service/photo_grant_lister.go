package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type photoGrantLister struct {
	photos gateway.PhotoGateway
	grants gateway.PhotoGrantGateway
	authz  usecase.Authorizer
}

func NewPhotoGrantLister(photos gateway.PhotoGateway, grants gateway.PhotoGrantGateway, authz usecase.Authorizer) usecase.PhotoGrantLister {
	return &photoGrantLister{photos: photos, grants: grants, authz: authz}
}

func (s *photoGrantLister) ListGrants(ctx context.Context, actor, tenantID, id string) ([]entity.Grant, error) {
	if err := requirePhotoShare(ctx, s.photos, s.authz, actor, tenantID, id); err != nil {
		return nil, err
	}
	return s.grants.List(ctx, id)
}
