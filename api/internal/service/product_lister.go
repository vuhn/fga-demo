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

type productLister struct {
	tenants  gateway.TenantGateway
	products gateway.ProductGateway
	authz    helper.Authorizer
}

func NewProductLister(tenants gateway.TenantGateway, products gateway.ProductGateway, authz helper.Authorizer) usecase.ProductLister {
	return &productLister{tenants: tenants, products: products, authz: authz}
}

// List shows product state on the tenant page: the internal admin
// (can_view_products) or the platform may read it.
func (s *productLister) List(ctx context.Context, in input.ProductLister) (output.ProductLister, error) {
	if err := s.authz.PlatformOr(ctx, in.Actor, "can_view_products", in.TenantID, model.ErrForbidden); err != nil {
		return output.ProductLister{}, err
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.ProductLister{}, err
	}
	got, err := s.products.List(ctx, in.TenantID)
	return output.ProductLister{Items: mapOutput(got, productOutput)}, err
}
