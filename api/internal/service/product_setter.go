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

type productSetter struct {
	tenants   gateway.TenantGateway
	products  gateway.ProductGateway
	relations gateway.ProductRelationGateway
	authz     helper.Authorizer
	guard     helper.WriteGuard
}

func NewProductSetter(tenants gateway.TenantGateway, products gateway.ProductGateway, relations gateway.ProductRelationGateway,
	authz helper.Authorizer, guard helper.WriteGuard) usecase.ProductSetter {
	return &productSetter{tenants: tenants, products: products, relations: relations, authz: authz, guard: guard}
}

// Set writes or deletes the single wildcard tuple. Content tuples are never
// touched. Products are an entitlement the platform grants: only a system
// admin may toggle them; tenant admins can only view them.
func (s *productSetter) Set(ctx context.Context, in input.ProductSetter) (output.ProductSetter, error) {
	if !model.ValidProduct(in.Product) {
		return output.ProductSetter{}, model.Invalid("product must be documents, photos, blueprints or tasks")
	}
	if err := s.authz.RequirePlatform(ctx, in.Actor); err != nil {
		return output.ProductSetter{}, err
	}
	if _, err := s.tenants.Get(ctx, in.TenantID); err != nil {
		return output.ProductSetter{}, err
	}
	wildcard := []model.Tuple{s.relations.Enabled(in.TenantID, in.Product)}
	var err error
	if in.Enabled {
		err = s.enable(ctx, in.TenantID, in.Product, wildcard)
	} else {
		err = s.disable(ctx, in.TenantID, in.Product, wildcard)
	}
	if err != nil {
		return output.ProductSetter{}, err
	}
	got, err := s.products.List(ctx, in.TenantID)
	return output.ProductSetter{Items: mapOutput(got, productOutput)}, err
}

func (s *productSetter) enable(ctx context.Context, tenantID, product string, wildcard []model.Tuple) error {
	prev, err := s.products.Set(ctx, tenantID, product, true)
	if err != nil {
		return err
	}
	return s.guard.Grant(ctx, wildcard, nil, func(ctx context.Context) error {
		_, err := s.products.Set(ctx, tenantID, product, prev)
		return err
	})
}

func (s *productSetter) disable(ctx context.Context, tenantID, product string, wildcard []model.Tuple) error {
	// Restore only when it was on before; the display flag tells us.
	products, err := s.products.List(ctx, tenantID)
	if err != nil {
		return err
	}
	var restore []model.Tuple
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
