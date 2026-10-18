package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type ProductLister interface {
	List(ctx context.Context, actor, tenantID string) ([]entity.Product, error)
}

type ProductSetter interface {
	Set(ctx context.Context, actor, tenantID, product string, enabled bool) ([]entity.Product, error)
}
