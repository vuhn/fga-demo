package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type productSetter struct {
	tenants   gateway.TenantGateway
	products  gateway.ProductGateway
	relations gateway.ProductRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewProductSetter(tenants gateway.TenantGateway, products gateway.ProductGateway, relations gateway.ProductRelationGateway,
	authz usecase.Authorizer, guard helper.WriteGuard) usecase.ProductSetter {
	return &productSetter{tenants: tenants, products: products, relations: relations, authz: authz, guard: guard}
}

// Set writes or deletes the single wildcard tuple. Content tuples are never
// touched. Products are an entitlement the platform grants: only a system
// admin may toggle them; tenant admins can only view them.
func (s *productSetter) Set(ctx context.Context, actor, tenantID, product string, enabled bool) ([]entity.Product, error) {
	if !entity.ValidProduct(product) {
		return nil, entity.Invalid("product must be documents or photos")
	}
	if err := s.authz.RequirePlatform(ctx, actor); err != nil {
		return nil, err
	}
	if _, err := s.tenants.Get(ctx, tenantID); err != nil {
		return nil, err
	}
	wildcard := []entity.Tuple{s.relations.Enabled(tenantID, product)}
	var err error
	if enabled {
		err = s.enable(ctx, tenantID, product, wildcard)
	} else {
		err = s.disable(ctx, tenantID, product, wildcard)
	}
	if err != nil {
		return nil, err
	}
	return s.products.List(ctx, tenantID)
}

func (s *productSetter) enable(ctx context.Context, tenantID, product string, wildcard []entity.Tuple) error {
	prev, err := s.products.Set(ctx, tenantID, product, true)
	if err != nil {
		return err
	}
	return s.guard.Grant(ctx, wildcard, nil, func(ctx context.Context) error {
		_, err := s.products.Set(ctx, tenantID, product, prev)
		return err
	})
}

func (s *productSetter) disable(ctx context.Context, tenantID, product string, wildcard []entity.Tuple) error {
	// Restore only when it was on before; the display flag tells us.
	products, err := s.products.List(ctx, tenantID)
	if err != nil {
		return err
	}
	var restore []entity.Tuple
	for _, p := range products {
		if p.Name == product && p.Enabled {
			restore = wildcard
		}
	}
	return s.guard.Revoke(ctx, wildcard, restore, func(ctx context.Context) error {
		_, err := s.products.Set(ctx, tenantID, product, false)
		return err
	})
}
