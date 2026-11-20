package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
	"github.com/vuhn/fga-demo/api/internal/usecase/input"
	"github.com/vuhn/fga-demo/api/internal/usecase/output"
)

type permissionReader struct {
	authz helper.Authorizer
}

func NewPermissionReader(authz helper.Authorizer) usecase.PermissionReader {
	return &permissionReader{authz: authz}
}

// Mine answers "what may I do here" in one BatchCheck per object. It only
// ever evaluates the actor, so it needs no authorization of its own.
func (s *permissionReader) Mine(ctx context.Context, in input.PermissionReader) (output.PermissionReader, error) {
	out := output.PermissionReader{TenantID: in.TenantID}
	if in.TenantID != "" {
		if err := model.RequireUUID("tenant_id", in.TenantID); err != nil {
			return out, err
		}
	}
	perms, err := s.authz.Permissions(ctx, in.Actor, model.ObjectTypePlatform, []string{model.PlatformID})
	if err != nil {
		return out, err
	}
	out.Platform = perms[model.PlatformID]
	if in.TenantID != "" {
		perms, err := s.authz.Permissions(ctx, in.Actor, model.ObjectTypeTenant, []string{in.TenantID})
		if err != nil {
			return out, err
		}
		out.Tenant = perms[in.TenantID]
	}
	return out, nil
}
