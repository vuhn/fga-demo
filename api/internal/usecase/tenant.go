package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// TenantView is a tenant plus the actor's display labels for it.
type TenantView struct {
	model.Tenant
	Affiliation string
	Role        string
	RoleName    string
}

type TenantLister interface {
	List(ctx context.Context, actor string) ([]model.Tenant, error)
}

// MyTenantLister lists the tenants the actor can read.
type MyTenantLister interface {
	ListMine(ctx context.Context, actor string) ([]TenantView, error)
}

type TenantCreator interface {
	Create(ctx context.Context, actor, name, slug string) (model.Tenant, error)
}

type TenantGetter interface {
	Get(ctx context.Context, actor, id string) (model.Tenant, error)
}

type TenantUpdater interface {
	UpdateName(ctx context.Context, actor, id, name string) (model.Tenant, error)
}
