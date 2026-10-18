package usecase

import (
	"context"
	"sort"

	"github.com/vuhn/fga-demo/internal/domain"
)

// Perms is the caller's own allow/deny for each permission relation on one
// object. It drives what the UI shows; the API still checks on every call.
type Perms map[string]bool

// MyPermissions is the caller's platform permissions and, when a tenant is
// given, their permissions on that tenant.
type MyPermissions struct {
	Platform Perms
	TenantID string
	Tenant   Perms // nil when no tenant was requested
}

// permissionRelations lists the relations reported for an object type, in a
// stable order. It reuses the inspector whitelist.
func permissionRelations(objectType string) []string {
	rels := make([]string, 0, len(relationsByType[objectType]))
	for r := range relationsByType[objectType] {
		rels = append(rels, r)
	}
	sort.Strings(rels)
	return rels
}

// MyPermissions answers "what may I do here" in one BatchCheck. It only ever
// evaluates the actor, so it needs no authorization of its own.
func (a *App) MyPermissions(ctx context.Context, actor, tenantID string) (MyPermissions, error) {
	out := MyPermissions{TenantID: tenantID}
	if tenantID != "" {
		if err := requireUUID("tenant_id", tenantID); err != nil {
			return out, err
		}
	}
	perms, err := a.objectPerms(ctx, actor, "platform", []string{domain.PlatformID})
	if err != nil {
		return out, err
	}
	out.Platform = perms[domain.PlatformID]
	if tenantID != "" {
		perms, err := a.objectPerms(ctx, actor, "tenant", []string{tenantID})
		if err != nil {
			return out, err
		}
		out.Tenant = perms[tenantID]
	}
	return out, nil
}

// objectPerms evaluates every permission relation of objectType for each id.
func (a *App) objectPerms(ctx context.Context, actor, objectType string, ids []string) (map[string]Perms, error) {
	rels := permissionRelations(objectType)
	checks := make([]CheckRequest, 0, len(ids)*len(rels))
	for _, id := range ids {
		for _, r := range rels {
			checks = append(checks, CheckRequest{UserID: actor, Relation: r, ObjectType: objectType, ObjectID: id})
		}
	}
	res, err := a.Authz.BatchCheck(ctx, checks)
	if err != nil {
		return nil, err
	}
	out := make(map[string]Perms, len(ids))
	for i, c := range checks {
		if out[c.ObjectID] == nil {
			out[c.ObjectID] = Perms{}
		}
		out[c.ObjectID][c.Relation] = res[i]
	}
	return out, nil
}

// Visible pairs an object the caller can view with the caller's permissions on it.
type Visible[T any] struct {
	Value T
	Perms Perms
}

func attachPerms[T any](ctx context.Context, a *App, actor, objectType string, xs []T, id func(T) string) ([]Visible[T], error) {
	out := make([]Visible[T], 0, len(xs))
	if len(xs) == 0 {
		return out, nil
	}
	ids := make([]string, len(xs))
	for i, x := range xs {
		ids[i] = id(x)
	}
	perms, err := a.objectPerms(ctx, actor, objectType, ids)
	if err != nil {
		return nil, err
	}
	for _, x := range xs {
		out = append(out, Visible[T]{Value: x, Perms: perms[id(x)]})
	}
	return out, nil
}

func containerID(c domain.Container) string { return c.ID }
func itemID(it domain.Item) string          { return it.ID }
