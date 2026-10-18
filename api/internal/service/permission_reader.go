package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/usecase"
)

type permissionReader struct {
	authz usecase.Authorizer
}

func NewPermissionReader(authz usecase.Authorizer) usecase.PermissionReader {
	return &permissionReader{authz: authz}
}

// Mine answers "what may I do here" in one BatchCheck per object. It only
// ever evaluates the actor, so it needs no authorization of its own.
func (s *permissionReader) Mine(ctx context.Context, actor, tenantID string) (usecase.MyPermissions, error) {
	out := usecase.MyPermissions{TenantID: tenantID}
	if tenantID != "" {
		if err := entity.RequireUUID("tenant_id", tenantID); err != nil {
			return out, err
		}
	}
	perms, err := s.authz.Permissions(ctx, actor, entity.ObjectTypePlatform, []string{entity.PlatformID})
	if err != nil {
		return out, err
	}
	out.Platform = perms[entity.PlatformID]
	if tenantID != "" {
		perms, err := s.authz.Permissions(ctx, actor, entity.ObjectTypeTenant, []string{tenantID})
		if err != nil {
			return out, err
		}
		out.Tenant = perms[tenantID]
	}
	return out, nil
}
