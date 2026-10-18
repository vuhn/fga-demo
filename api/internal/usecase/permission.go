package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// MyPermissions is the caller's platform permissions and, when a tenant is
// given, their permissions on that tenant.
type MyPermissions struct {
	Platform entity.Perms
	TenantID string
	Tenant   entity.Perms // nil when no tenant was requested
}

// PermissionReader answers "what may I do here" for the caller only.
type PermissionReader interface {
	Mine(ctx context.Context, actor, tenantID string) (MyPermissions, error)
}
