package usecase

import (
	"context"

	"github.com/vuhn/fga-demo/internal/domain"
)

// relationsByType is the inspector whitelist.
var relationsByType = map[string]map[string]bool{
	"platform": set("can_manage_platform"),
	"tenant": set("can_manage_members", "can_view_products", "can_contribute", "can_read_tenant",
		"can_create_folder", "can_create_album", "can_create_private_folder", "can_create_private_album",
		"documents_enabled", "photos_enabled"),
	"folder":   set("can_view", "can_edit", "can_manage", "can_delete", "can_share", "can_change_visibility", "can_create_document"),
	"document": set("can_view", "can_edit", "can_manage", "can_delete", "can_share"),
	"album":    set("can_view", "can_edit", "can_manage", "can_delete", "can_share", "can_change_visibility", "can_create_photo"),
	"photo":    set("can_view", "can_edit", "can_manage", "can_delete", "can_share"),
}

func set(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func validateInspect(c CheckRequest) error {
	rels, ok := relationsByType[c.ObjectType]
	if !ok {
		return domain.Invalid("unknown object_type %q", c.ObjectType)
	}
	if !rels[c.Relation] {
		return domain.Invalid("relation %q is not allowed on %s", c.Relation, c.ObjectType)
	}
	if err := requireUUID("user_id", c.UserID); err != nil {
		return err
	}
	if c.ObjectType == "platform" {
		if c.ObjectID != domain.PlatformID {
			return domain.Invalid("platform object_id must be main")
		}
		return nil
	}
	return requireUUID("object_id", c.ObjectID)
}

// inspectAuth decides whether actor may ask about other users on objects.
// It caches the platform Check and per-tenant can_manage_members.
type inspectAuth struct {
	a        *App
	actor    string
	platform *bool
	tenants  map[string]bool
}

func (ia *inspectAuth) allowed(ctx context.Context, c CheckRequest) error {
	if c.UserID == ia.actor {
		return nil
	}
	if ia.platform == nil {
		ok, err := ia.a.check(ctx, ia.actor, "can_manage_platform", "platform", domain.PlatformID)
		if err != nil {
			return err
		}
		ia.platform = &ok
	}
	if *ia.platform {
		return nil
	}
	// The tenant comes from the object, never from the request.
	var tenantID string
	switch c.ObjectType {
	case "platform":
		return denied(domain.ErrForbidden, "can_manage_platform", "platform", domain.PlatformID)
	case "tenant":
		tenantID = c.ObjectID
	default:
		t, err := ia.a.objectTenant(ctx, c.ObjectType, c.ObjectID)
		if err != nil {
			return err
		}
		tenantID = t
	}
	ok, seen := ia.tenants[tenantID]
	if !seen {
		var err error
		if ok, err = ia.a.check(ctx, ia.actor, "can_manage_members", "tenant", tenantID); err != nil {
			return err
		}
		ia.tenants[tenantID] = ok
	}
	if !ok {
		return denied(domain.ErrForbidden, "can_manage_members", "tenant", tenantID)
	}
	return nil
}

// BatchInspect evaluates up to 50 checks after authorizing each item.
func (a *App) BatchInspect(ctx context.Context, actor string, checks []CheckRequest) ([]bool, error) {
	if len(checks) == 0 {
		return []bool{}, nil
	}
	if len(checks) > 50 {
		return nil, domain.Invalid("at most 50 checks")
	}
	ia := &inspectAuth{a: a, actor: actor, tenants: map[string]bool{}}
	for _, c := range checks {
		if err := validateInspect(c); err != nil {
			return nil, err
		}
		if err := ia.allowed(ctx, c); err != nil {
			return nil, err
		}
	}
	return a.Authz.BatchCheck(ctx, checks)
}

func (a *App) Inspect(ctx context.Context, actor string, c CheckRequest) (bool, error) {
	res, err := a.BatchInspect(ctx, actor, []CheckRequest{c})
	if err != nil {
		return false, err
	}
	return res[0], nil
}

type MemberAccess struct {
	UserID    string
	Name      string
	CanView   bool
	CanEdit   bool
	CanDelete bool
}

type AccessReport struct {
	Object       string
	DirectTuples []Tuple
	Members      []MemberAccess
}

// Access lists the direct tuples on an object and evaluates every tenant
// member plus every platform admin with BatchCheck.
func (a *App) Access(ctx context.Context, actor, objectType, objectID string) (AccessReport, error) {
	if _, ok := domain.KindOf(objectType); !ok {
		return AccessReport{}, domain.Invalid("object_type must be folder, document, album, or photo")
	}
	tenantID, err := a.objectTenant(ctx, objectType, objectID)
	if err != nil {
		return AccessReport{}, err
	}
	allowed := false
	for _, c := range []CheckRequest{
		{actor, "can_manage_platform", "platform", domain.PlatformID},
		{actor, "can_share", objectType, objectID},
		{actor, "can_manage_members", "tenant", tenantID},
	} {
		ok, err := a.check(ctx, c.UserID, c.Relation, c.ObjectType, c.ObjectID)
		if err != nil {
			return AccessReport{}, err
		}
		if ok {
			allowed = true
			break
		}
	}
	if !allowed {
		return AccessReport{}, denied(domain.ErrForbidden, "can_share", objectType, objectID)
	}

	direct, err := a.Authz.ReadTuples(ctx, objectType, objectID)
	if err != nil {
		return AccessReport{}, err
	}
	members, err := a.Store.ListMembers(ctx, tenantID, "")
	if err != nil {
		return AccessReport{}, err
	}
	admins, err := a.Store.ListPlatformAdmins(ctx)
	if err != nil {
		return AccessReport{}, err
	}
	people := []MemberAccess{}
	seen := map[string]bool{}
	for _, m := range members {
		people = append(people, MemberAccess{UserID: m.UserID, Name: m.Name})
		seen[m.UserID] = true
	}
	for _, u := range admins {
		if !seen[u.ID] {
			people = append(people, MemberAccess{UserID: u.ID, Name: u.Name})
		}
	}
	rels := []string{"can_view", "can_edit", "can_delete"}
	var checks []CheckRequest
	for _, p := range people {
		for _, r := range rels {
			checks = append(checks, CheckRequest{p.UserID, r, objectType, objectID})
		}
	}
	res, err := a.Authz.BatchCheck(ctx, checks)
	if err != nil {
		return AccessReport{}, err
	}
	for i := range people {
		people[i].CanView, people[i].CanEdit, people[i].CanDelete = res[3*i], res[3*i+1], res[3*i+2]
	}
	if direct == nil {
		direct = []Tuple{}
	}
	return AccessReport{Object: objectType + ":" + objectID, DirectTuples: direct, Members: people}, nil
}
