package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

type RoleLister interface {
	List(ctx context.Context, actor, tenantID string) ([]entity.Role, error)
}

type RoleGetter interface {
	Get(ctx context.Context, actor, tenantID, roleID string) (entity.Role, error)
}

type RoleCreator interface {
	Create(ctx context.Context, actor, tenantID, name, affiliation string) (entity.Role, error)
}

type RoleUpdater interface {
	UpdateName(ctx context.Context, actor, tenantID, roleID, name string) (entity.Role, error)
}

type RolePermissionSetter interface {
	SetPermissions(ctx context.Context, actor, tenantID, roleID string, permissions []string) (entity.Role, error)
}

type RoleDeleter interface {
	Delete(ctx context.Context, actor, tenantID, roleID string) error
}
