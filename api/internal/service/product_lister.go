package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type productLister struct {
	tenants  gateway.TenantGateway
	products gateway.ProductGateway
	authz    usecase.Authorizer
}

func NewProductLister(tenants gateway.TenantGateway, products gateway.ProductGateway, authz usecase.Authorizer) usecase.ProductLister {
	return &productLister{tenants: tenants, products: products, authz: authz}
}

// List shows product state on the tenant page: the internal admin
// (can_view_products) or the platform may read it.
func (s *productLister) List(ctx context.Context, actor, tenantID string) ([]entity.Product, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_view_products", tenantID, entity.ErrForbidden); err != nil {
		return nil, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return nil, err
	}
	return s.products.List(ctx, tenantID)
}
