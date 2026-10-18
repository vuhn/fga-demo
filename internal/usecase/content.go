package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/vuhn/fga-demo/internal/domain"
)

// Documents and photos share every rule. Each method takes the Kind that names
// the object types and relations; the HTTP layer mounts it twice.

const maxDepth = 10

// ---- Containers (folders, albums) ----

// ListContainers returns the visible containers with the caller's permissions
// on each, so the UI needs no extra Check round trips.
func (a *App) ListContainers(ctx context.Context, actor, tenantID string, kind domain.Kind) ([]Visible[domain.Container], error) {
	ids, err := a.Authz.ListObjectIDs(ctx, actor, "can_view", kind.Container)
	if err != nil || len(ids) == 0 {
		return []Visible[domain.Container]{}, err
	}
	cs, err := a.Store.ContainersByIDs(ctx, kind, tenantID, ids)
	if err != nil {
		return nil, err
	}
	return attachPerms(ctx, a, actor, kind.Container, cs, containerID)
}

// CreateContainer creates a folder or album. A private container needs the
// tenant's can_create_private_* (internal members); a subfolder also needs
// can_edit on its parent, so an external admin can only nest inside content
// they can already work in.
func (a *App) CreateContainer(ctx context.Context, actor, tenantID string, kind domain.Kind, name string, parentID *string, isPrivate bool) (domain.Container, error) {
	name, err := cleanName("name", name)
	if err != nil {
		return domain.Container{}, err
	}
	if err := a.require(ctx, actor, kind.CreateCont, "tenant", tenantID); err != nil {
		return domain.Container{}, err
	}
	if isPrivate {
		if err := a.require(ctx, actor, kind.CreatePrivate, "tenant", tenantID); err != nil {
			return domain.Container{}, err
		}
	}
	if parentID != nil {
		if err := requireUUID("parent_id", *parentID); err != nil {
			return domain.Container{}, err
		}
		if err := a.checkDepth(ctx, tenantID, kind, *parentID); err != nil {
			return domain.Container{}, err
		}
		if err := a.require(ctx, actor, "can_edit", kind.Container, *parentID); err != nil {
			return domain.Container{}, err
		}
	}
	c := domain.Container{ID: a.NewID(), TenantID: tenantID, ParentID: parentID, Name: name, CreatedBy: actor, IsPrivate: isPrivate}
	if err := a.Store.CreateContainer(ctx, kind, c, a.ownerGrant(tenantID, kind.Container, c.ID, actor)); err != nil {
		return domain.Container{}, err
	}
	err = a.grant(ctx, a.Tuples.CreateContainer(kind, tenantID, c.ID, actor, parentID, isPrivate), nil, func(ctx context.Context) error {
		return a.Store.DeleteObject(ctx, kind.Container, c.ID)
	})
	if err != nil {
		return domain.Container{}, err
	}
	return a.Store.GetContainer(ctx, kind, c.ID)
}

// SetVisibility makes a container private or public. Making it public grants
// external members access (a grant); making it private removes it (a revoke).
func (a *App) SetVisibility(ctx context.Context, actor, tenantID string, kind domain.Kind, id string, isPrivate bool) (domain.Container, error) {
	c, err := a.loadContainer(ctx, tenantID, kind, id)
	if err != nil {
		return domain.Container{}, err
	}
	if err := a.require(ctx, actor, "can_change_visibility", kind.Container, id); err != nil {
		return domain.Container{}, err
	}
	if c.IsPrivate == isPrivate {
		return c, nil
	}
	writes, deletes := a.Tuples.SetVisibility(kind, tenantID, id, isPrivate)
	if isPrivate {
		err = a.revoke(ctx, deletes, deletes, func(ctx context.Context) error {
			_, err := a.Store.SetContainerPrivate(ctx, kind, id, true)
			return err
		})
	} else {
		if _, err := a.Store.SetContainerPrivate(ctx, kind, id, false); err != nil {
			return domain.Container{}, err
		}
		err = a.grant(ctx, writes, nil, func(ctx context.Context) error {
			_, err := a.Store.SetContainerPrivate(ctx, kind, id, true)
			return err
		})
	}
	if err != nil {
		return domain.Container{}, err
	}
	return a.Store.GetContainer(ctx, kind, id)
}

// checkDepth walks parent_id in Postgres. The parent must be in the header
// tenant; the chain must be acyclic and at most maxDepth long.
func (a *App) checkDepth(ctx context.Context, tenantID string, kind domain.Kind, parentID string) error {
	seen := map[string]bool{}
	id := &parentID
	for depth := 1; id != nil; depth++ {
		if depth > maxDepth {
			return fmt.Errorf("%w: %s depth exceeds %d", domain.ErrConflict, kind.Container, maxDepth)
		}
		if seen[*id] {
			return fmt.Errorf("%w: %s cycle", domain.ErrConflict, kind.Container)
		}
		seen[*id] = true
		c, err := a.Store.GetContainer(ctx, kind, *id)
		if err != nil {
			return err
		}
		if c.TenantID != tenantID {
			return domain.ErrNotFound
		}
		id = c.ParentID
	}
	return nil
}

func (a *App) loadContainer(ctx context.Context, tenantID string, kind domain.Kind, id string) (domain.Container, error) {
	if err := requireUUID("id", id); err != nil {
		return domain.Container{}, err
	}
	c, err := a.Store.GetContainer(ctx, kind, id)
	if err != nil {
		return domain.Container{}, err
	}
	if c.TenantID != tenantID {
		return domain.Container{}, domain.ErrNotFound
	}
	return c, nil
}

func (a *App) GetContainer(ctx context.Context, actor, tenantID string, kind domain.Kind, id string) (Visible[domain.Container], error) {
	c, err := a.loadContainer(ctx, tenantID, kind, id)
	if err != nil {
		return Visible[domain.Container]{}, err
	}
	if err := a.requireVisible(ctx, actor, "can_view", kind.Container, id); err != nil {
		return Visible[domain.Container]{}, err
	}
	vs, err := attachPerms(ctx, a, actor, kind.Container, []domain.Container{c}, containerID)
	if err != nil {
		return Visible[domain.Container]{}, err
	}
	return vs[0], nil
}

// RenameContainer requires can_manage: a userset editor can fill a library but
// cannot rename it.
func (a *App) RenameContainer(ctx context.Context, actor, tenantID string, kind domain.Kind, id, name string) (domain.Container, error) {
	if _, err := a.loadContainer(ctx, tenantID, kind, id); err != nil {
		return domain.Container{}, err
	}
	name, err := cleanName("name", name)
	if err != nil {
		return domain.Container{}, err
	}
	if err := a.require(ctx, actor, "can_manage", kind.Container, id); err != nil {
		return domain.Container{}, err
	}
	return a.Store.RenameContainer(ctx, kind, id, name)
}

func (a *App) DeleteContainer(ctx context.Context, actor, tenantID string, kind domain.Kind, id string) error {
	if _, err := a.loadContainer(ctx, tenantID, kind, id); err != nil {
		return err
	}
	if err := a.require(ctx, actor, "can_delete", kind.Container, id); err != nil {
		return err
	}
	has, err := a.Store.ContainerHasChildren(ctx, kind, id)
	if err != nil {
		return err
	}
	if has {
		return fmt.Errorf("%w: %s still has children", domain.ErrConflict, kind.Container)
	}
	return a.deleteObject(ctx, kind.Container, id)
}

// deleteObject removes every tuple on the object, then the row and its grants.
func (a *App) deleteObject(ctx context.Context, objectType, id string) error {
	tuples, err := a.Authz.ReadTuples(ctx, objectType, id)
	if err != nil {
		return err
	}
	return a.revoke(ctx, tuples, tuples, func(ctx context.Context) error {
		return a.Store.DeleteObject(ctx, objectType, id)
	})
}

func (a *App) ownerGrant(tenantID, objectType, objectID, userID string) domain.Grant {
	return domain.Grant{ID: a.NewID(), TenantID: tenantID, ObjectType: objectType, ObjectID: objectID,
		Relation: domain.RelOwner, SubjectType: domain.SubjectUser, UserID: &userID}
}

// ---- Items (documents, photos) ----

// ListItems returns every visible item in the tenant, including items whose
// container the actor cannot see ("Shared with me").
func (a *App) ListItems(ctx context.Context, actor, tenantID string, kind domain.Kind, containerID string) ([]Visible[domain.Item], error) {
	if containerID != "" {
		if err := requireUUID(kind.ContainerRef+"_id", containerID); err != nil {
			return nil, err
		}
	}
	ids, err := a.Authz.ListObjectIDs(ctx, actor, "can_view", kind.Item)
	if err != nil || len(ids) == 0 {
		return []Visible[domain.Item]{}, err
	}
	its, err := a.Store.ItemsByIDs(ctx, kind, tenantID, ids, containerID)
	if err != nil {
		return nil, err
	}
	return attachPerms(ctx, a, actor, kind.Item, its, itemID)
}

type NewItem struct {
	ContainerID string
	Name        string
	MimeType    string
	ByteSize    int64
}

// CreateItem is the upload command. It stores metadata only.
func (a *App) CreateItem(ctx context.Context, actor, tenantID string, kind domain.Kind, in NewItem) (domain.Item, error) {
	name, err := cleanName("name", in.Name)
	if err != nil {
		return domain.Item{}, err
	}
	mime := strings.TrimSpace(in.MimeType)
	if n := len(mime); n < 1 || n > 200 {
		return domain.Item{}, domain.Invalid("mime_type length must be 1 to 200")
	}
	if in.ByteSize < 0 {
		return domain.Item{}, domain.Invalid("byte_size must be >= 0")
	}
	if _, err := a.loadContainer(ctx, tenantID, kind, in.ContainerID); err != nil {
		return domain.Item{}, err
	}
	if err := a.require(ctx, actor, kind.CreateItem, kind.Container, in.ContainerID); err != nil {
		return domain.Item{}, err
	}
	it := domain.Item{ID: a.NewID(), TenantID: tenantID, ContainerID: in.ContainerID, Name: name, MimeType: mime, ByteSize: in.ByteSize, CreatedBy: actor}
	if err := a.Store.CreateItem(ctx, kind, it, a.ownerGrant(tenantID, kind.Item, it.ID, actor)); err != nil {
		return domain.Item{}, err
	}
	err = a.grant(ctx, a.Tuples.CreateItem(kind, tenantID, it.ID, in.ContainerID, actor), nil, func(ctx context.Context) error {
		return a.Store.DeleteObject(ctx, kind.Item, it.ID)
	})
	if err != nil {
		return domain.Item{}, err
	}
	return a.Store.GetItem(ctx, kind, it.ID)
}

func (a *App) loadItem(ctx context.Context, tenantID string, kind domain.Kind, id string) (domain.Item, error) {
	if err := requireUUID("id", id); err != nil {
		return domain.Item{}, err
	}
	it, err := a.Store.GetItem(ctx, kind, id)
	if err != nil {
		return domain.Item{}, err
	}
	if it.TenantID != tenantID {
		return domain.Item{}, domain.ErrNotFound
	}
	return it, nil
}

func (a *App) GetItem(ctx context.Context, actor, tenantID string, kind domain.Kind, id string) (Visible[domain.Item], error) {
	it, err := a.loadItem(ctx, tenantID, kind, id)
	if err != nil {
		return Visible[domain.Item]{}, err
	}
	if err := a.requireVisible(ctx, actor, "can_view", kind.Item, id); err != nil {
		return Visible[domain.Item]{}, err
	}
	vs, err := attachPerms(ctx, a, actor, kind.Item, []domain.Item{it}, itemID)
	if err != nil {
		return Visible[domain.Item]{}, err
	}
	return vs[0], nil
}

func (a *App) RenameItem(ctx context.Context, actor, tenantID string, kind domain.Kind, id, name string) (domain.Item, error) {
	if _, err := a.loadItem(ctx, tenantID, kind, id); err != nil {
		return domain.Item{}, err
	}
	name, err := cleanName("name", name)
	if err != nil {
		return domain.Item{}, err
	}
	if err := a.require(ctx, actor, "can_edit", kind.Item, id); err != nil {
		return domain.Item{}, err
	}
	return a.Store.RenameItem(ctx, kind, id, name)
}

func (a *App) DeleteItem(ctx context.Context, actor, tenantID string, kind domain.Kind, id string) error {
	if _, err := a.loadItem(ctx, tenantID, kind, id); err != nil {
		return err
	}
	if err := a.require(ctx, actor, "can_delete", kind.Item, id); err != nil {
		return err
	}
	return a.deleteObject(ctx, kind.Item, id)
}

// ---- Grants (share and revoke) ----

// objectTenant loads a content object and returns its tenant.
func (a *App) objectTenant(ctx context.Context, objectType, id string) (string, error) {
	if err := requireUUID("id", id); err != nil {
		return "", err
	}
	kind, ok := domain.KindOf(objectType)
	if !ok {
		return "", domain.Invalid("unknown object type %q", objectType)
	}
	if objectType == kind.Container {
		c, err := a.Store.GetContainer(ctx, kind, id)
		return c.TenantID, err
	}
	it, err := a.Store.GetItem(ctx, kind, id)
	return it.TenantID, err
}

func (a *App) loadShareable(ctx context.Context, actor, tenantID, objectType, id string) error {
	t, err := a.objectTenant(ctx, objectType, id)
	if err != nil {
		return err
	}
	if t != tenantID {
		return domain.ErrNotFound
	}
	return a.require(ctx, actor, "can_share", objectType, id)
}

func (a *App) ListGrants(ctx context.Context, actor, tenantID, objectType, id string) ([]domain.Grant, error) {
	if err := a.loadShareable(ctx, actor, tenantID, objectType, id); err != nil {
		return nil, err
	}
	return a.Store.ListGrants(ctx, objectType, id)
}

// ShareCandidates lists the people a sharer can pick: the external members of
// the object's tenant. It needs can_share on that object, not member
// management, so owners can share without seeing the full roster.
func (a *App) ShareCandidates(ctx context.Context, actor, tenantID, objectType, id string) ([]domain.Membership, error) {
	if err := a.loadShareable(ctx, actor, tenantID, objectType, id); err != nil {
		return nil, err
	}
	return a.Store.ListMembers(ctx, tenantID, domain.AffiliationExternal)
}

type ShareInput struct {
	Relation string
	Subject  string
	UserID   string
}

func (a *App) shareGrant(tenantID, objectType, id string, in ShareInput) (domain.Grant, error) {
	switch in.Relation {
	case domain.RelViewer, domain.RelEditor:
	case domain.RelOwner:
		return domain.Grant{}, domain.Invalid("owner cannot be granted or revoked")
	default:
		return domain.Grant{}, domain.Invalid("relation must be viewer or editor")
	}
	g := domain.Grant{TenantID: tenantID, ObjectType: objectType, ObjectID: id, Relation: in.Relation, SubjectType: in.Subject}
	if in.Subject == domain.SubjectUser {
		if err := requireUUID("user_id", in.UserID); err != nil {
			return domain.Grant{}, err
		}
		uid := in.UserID
		g.UserID = &uid
	} else if in.UserID != "" {
		return domain.Grant{}, domain.Invalid("user_id is only allowed with subject user")
	}
	return g, nil
}

func sameSubject(a, b domain.Grant) bool {
	if a.SubjectType != b.SubjectType {
		return false
	}
	if a.SubjectType != domain.SubjectUser {
		return true
	}
	return a.UserID != nil && b.UserID != nil && *a.UserID == *b.UserID
}

func (a *App) Share(ctx context.Context, actor, tenantID, objectType, id string, in ShareInput) (domain.Grant, error) {
	g, err := a.shareGrant(tenantID, objectType, id, in)
	if err != nil {
		return domain.Grant{}, err
	}
	tuple, err := a.Tuples.Grant(g) // rejects editor + external_users
	if err != nil {
		return domain.Grant{}, err
	}
	if err := a.loadShareable(ctx, actor, tenantID, objectType, id); err != nil {
		return domain.Grant{}, err
	}
	if g.UserID != nil {
		ok, err := a.check(ctx, *g.UserID, "can_read_tenant", "tenant", tenantID)
		if err != nil {
			return domain.Grant{}, err
		}
		if !ok {
			return domain.Grant{}, domain.ErrNotAMember
		}
		if g.Relation == domain.RelEditor {
			ok, err := a.check(ctx, *g.UserID, "can_contribute", "tenant", tenantID)
			if err != nil {
				return domain.Grant{}, err
			}
			if !ok {
				return domain.Grant{}, domain.ErrCannotUpload
			}
		}
	}
	existing, err := a.Store.ListGrants(ctx, objectType, id)
	if err != nil {
		return domain.Grant{}, err
	}
	for _, e := range existing {
		if !sameSubject(e, g) {
			continue
		}
		if e.Relation == domain.RelOwner {
			return domain.Grant{}, domain.Invalid("the owner grant cannot be changed")
		}
		if e.Relation == g.Relation {
			return e, nil
		}
	}

	g.ID = a.NewID()
	prev, err := a.Store.UpsertGrant(ctx, g)
	if err != nil {
		return domain.Grant{}, err
	}
	var deletes []Tuple
	if prev != nil {
		old := g
		old.Relation = prev.Relation
		t, err := a.Tuples.Grant(old)
		if err != nil {
			return domain.Grant{}, err
		}
		deletes = append(deletes, t)
		g.ID = prev.ID
	}
	err = a.grant(ctx, []Tuple{tuple}, deletes, func(ctx context.Context) error {
		if prev == nil {
			return a.Store.DeleteGrant(ctx, g.ID)
		}
		return a.Store.RestoreGrant(ctx, *prev)
	})
	return g, err
}

func (a *App) Revoke(ctx context.Context, actor, tenantID, objectType, id string, in ShareInput) error {
	g, err := a.shareGrant(tenantID, objectType, id, in)
	if err != nil {
		return err
	}
	if err := a.loadShareable(ctx, actor, tenantID, objectType, id); err != nil {
		return err
	}
	existing, err := a.Store.ListGrants(ctx, objectType, id)
	if err != nil {
		return err
	}
	for _, e := range existing {
		if !sameSubject(e, g) || e.Relation != g.Relation {
			continue
		}
		t, err := a.Tuples.Grant(e)
		if err != nil {
			return err
		}
		return a.revoke(ctx, []Tuple{t}, []Tuple{t}, func(ctx context.Context) error {
			return a.Store.DeleteGrant(ctx, e.ID)
		})
	}
	return domain.ErrNotFound
}
