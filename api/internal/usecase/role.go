package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type RoleLister interface {
	List(ctx context.Context, actor, tenantID string) ([]model.Role, error)
}

type RoleGetter interface {
	Get(ctx context.Context, actor, tenantID, roleID string) (model.Role, error)
}

type RoleCreator interface {
	Create(ctx context.Context, actor, tenantID, name, affiliation string) (model.Role, error)
}

type RoleUpdater interface {
	UpdateName(ctx context.Context, actor, tenantID, roleID, name string) (model.Role, error)
}

type RolePermissionSetter interface {
	SetPermissions(ctx context.Context, actor, tenantID, roleID string, permissions []string) (model.Role, error)
}

type RoleDeleter interface {
	Delete(ctx context.Context, actor, tenantID, roleID string) error
}
