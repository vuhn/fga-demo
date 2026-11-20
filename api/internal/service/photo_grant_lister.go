package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type photoGrantLister struct {
	photos gateway.PhotoGateway
	grants gateway.PhotoGrantGateway
	authz  helper.Authorizer
}

func NewPhotoGrantLister(photos gateway.PhotoGateway, grants gateway.PhotoGrantGateway, authz helper.Authorizer) usecase.PhotoGrantLister {
	return &photoGrantLister{photos: photos, grants: grants, authz: authz}
}

func (s *photoGrantLister) ListGrants(ctx context.Context, in input.PhotoGrantLister) (output.PhotoGrantLister, error) {
	if err := requirePhotoShare(ctx, s.photos, s.authz, in.Actor, in.TenantID, in.ID); err != nil {
		return output.PhotoGrantLister{}, err
	}
	got, err := s.grants.List(ctx, in.ID)
	return output.PhotoGrantLister{Items: mapOutput(got, grantOutput)}, err
}
