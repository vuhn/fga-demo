package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type TenantLister interface {
	List(ctx context.Context, in input.TenantLister) (output.TenantLister, error)
}

// MyTenantLister lists the tenants the actor can read.
type MyTenantLister interface {
	ListMine(ctx context.Context, in input.MyTenantLister) (output.MyTenantLister, error)
}

type TenantCreator interface {
	Create(ctx context.Context, in input.TenantCreator) (output.TenantCreator, error)
}

type TenantGetter interface {
	Get(ctx context.Context, in input.TenantGetter) (output.TenantGetter, error)
}

type TenantUpdater interface {
	UpdateName(ctx context.Context, in input.TenantUpdater) (output.TenantUpdater, error)
}
