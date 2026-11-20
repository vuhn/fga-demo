package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type RoleLister interface {
	List(ctx context.Context, in input.RoleLister) (output.RoleLister, error)
}

type RoleGetter interface {
	Get(ctx context.Context, in input.RoleGetter) (output.RoleGetter, error)
}

type RoleCreator interface {
	Create(ctx context.Context, in input.RoleCreator) (output.RoleCreator, error)
}

type RoleUpdater interface {
	UpdateName(ctx context.Context, in input.RoleUpdater) (output.RoleUpdater, error)
}

type RolePermissionSetter interface {
	SetPermissions(ctx context.Context, in input.RolePermissionSetter) (output.RolePermissionSetter, error)
}

type RoleDeleter interface {
	Delete(ctx context.Context, in input.RoleDeleter) error
}
