package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type ProductLister interface {
	List(ctx context.Context, actor, tenantID string) ([]model.Product, error)
}

type ProductSetter interface {
	Set(ctx context.Context, actor, tenantID, product string, enabled bool) ([]model.Product, error)
}
