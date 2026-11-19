package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

const maxInspectChecks = 50

type inspector struct {
	authz   usecase.Authorizer
	tenants helper.ObjectTenantResolver
}

func NewInspector(authz usecase.Authorizer, tenants helper.ObjectTenantResolver) usecase.Inspector {
	return &inspector{authz: authz, tenants: tenants}
}

func (s *inspector) Inspect(ctx context.Context, actor string, c model.CheckRequest) (bool, error) {
	res, err := s.BatchInspect(ctx, actor, []model.CheckRequest{c})
	if err != nil {
		return false, err
	}
	return res[0], nil
}

// BatchInspect evaluates up to 50 checks after authorizing each item.
func (s *inspector) BatchInspect(ctx context.Context, actor string, checks []model.CheckRequest) ([]bool, error) {
	if len(checks) == 0 {
		return []bool{}, nil
	}
	if len(checks) > maxInspectChecks {
		return nil, model.Invalid("at most 50 checks")
	}
	grant := &inspectGrant{authz: s.authz, tenants: s.tenants, actor: actor, managed: map[string]bool{}}
	for _, c := range checks {
		if err := validateInspect(c); err != nil {
			return nil, err
		}
		if err := grant.allowed(ctx, c); err != nil {
			return nil, err
		}
	}
	return s.authz.BatchCheck(ctx, checks)
}

func validateInspect(c model.CheckRequest) error {
	if _, ok := model.PermissionRelations[c.ObjectType]; !ok {
		return model.Invalid("unknown object_type %q", c.ObjectType)
	}
	if !model.IsPermissionRelation(c.ObjectType, c.Relation) {
		return model.Invalid("relation %q is not allowed on %s", c.Relation, c.ObjectType)
	}
	if err := model.RequireUUID("user_id", c.UserID); err != nil {
		return err
	}
	if c.ObjectType == model.ObjectTypePlatform {
		if c.ObjectID != model.PlatformID {
			return model.Invalid("platform object_id must be main")
		}
		return nil
	}
	return model.RequireUUID("object_id", c.ObjectID)
}

// inspectGrant decides whether actor may ask about other users on objects.
// It caches the platform Check and per-tenant can_manage_members.
type inspectGrant struct {
	authz    usecase.Authorizer
	tenants  helper.ObjectTenantResolver
	actor    string
	platform *bool
	managed  map[string]bool
}

func (g *inspectGrant) allowed(ctx context.Context, c model.CheckRequest) error {
	if c.UserID == g.actor {
		return nil
	}
	if g.platform == nil {
		ok, err := g.authz.IsPlatformAdmin(ctx, g.actor)
		if err != nil {
			return err
		}
		g.platform = &ok
	}
	if *g.platform {
		return nil
	}
	// The tenant comes from the object, never from the request.
	var tenantID string
	switch c.ObjectType {
	case model.ObjectTypePlatform:
		return model.Denied(model.ErrForbidden, "can_manage_platform", model.ObjectTypePlatform, model.PlatformID)
	case model.ObjectTypeTenant:
		tenantID = c.ObjectID
	default:
		t, err := g.tenants.TenantOf(ctx, c.ObjectType, c.ObjectID)
		if err != nil {
			return err
		}
		tenantID = t
	}
	ok, seen := g.managed[tenantID]
	if !seen {
		var err error
		if ok, err = g.authz.Check(ctx, g.actor, "can_manage_members", model.ObjectTypeTenant, tenantID); err != nil {
			return err
		}
		g.managed[tenantID] = ok
	}
	if !ok {
		return model.Denied(model.ErrForbidden, "can_manage_members", model.ObjectTypeTenant, tenantID)
	}
	return nil
}
