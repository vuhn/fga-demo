package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// TenantView is a tenant plus the actor's display labels for it.
type TenantView struct {
	entity.Tenant
	Affiliation string
	Role        string
	RoleName    string
}

type TenantLister interface {
	List(ctx context.Context, actor string) ([]entity.Tenant, error)
}

// MyTenantLister lists the tenants the actor can read.
type MyTenantLister interface {
	ListMine(ctx context.Context, actor string) ([]TenantView, error)
}

type TenantCreator interface {
	Create(ctx context.Context, actor, name, slug string) (entity.Tenant, error)
}

type TenantGetter interface {
	Get(ctx context.Context, actor, id string) (entity.Tenant, error)
}

type TenantUpdater interface {
	UpdateName(ctx context.Context, actor, id, name string) (entity.Tenant, error)
}
