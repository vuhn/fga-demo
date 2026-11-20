package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type ProductLister interface {
	List(ctx context.Context, in input.ProductLister) (output.ProductLister, error)
}

type ProductSetter interface {
	Set(ctx context.Context, in input.ProductSetter) (output.ProductSetter, error)
}
