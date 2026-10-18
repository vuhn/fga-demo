package openfga

import (
	"github.com/vuhn/fga-demo/internal/domain"
	"github.com/vuhn/fga-demo/internal/usecase"
)

// Catalog builds the tuples for each domain command. Every method is pure.
type Catalog struct{}

var _ usecase.TupleCatalog = Catalog{}

func obj(typ, id string) string           { return typ + ":" + id }
func userRef(id string) string            { return "user:" + id }
func userset(tenantID, rel string) string { return "tenant:" + tenantID + "#" + rel }

const wildcard = "user:*"

// PlatformAdmin is written only by the seed.
func PlatformAdmin(userID string) usecase.Tuple {
	return usecase.Tuple{User: userRef(userID), Relation: "system_admin", Object: obj("platform", domain.PlatformID)}
}

func (Catalog) SetProduct(tenantID, product string, enabled bool) (writes, deletes []usecase.Tuple) {
	t := usecase.Tuple{User: wildcard, Relation: product + "_enabled", Object: obj("tenant", tenantID)}
	if enabled {
		return []usecase.Tuple{t}, nil
	}
	return nil, []usecase.Tuple{t}
}

// AssignRole writes the new role relation and deletes the other three, so a
// person holds exactly one direct role on the tenant.
func (Catalog) AssignRole(tenantID, userID, affiliation, role string) (writes, deletes []usecase.Tuple, err error) {
	if err := domain.CheckAffiliationRole(affiliation, role); err != nil {
		return nil, nil, err
	}
	for _, r := range domain.AllRoles {
		t := usecase.Tuple{User: userRef(userID), Relation: r, Object: obj("tenant", tenantID)}
		if r == role {
			writes = append(writes, t)
		} else {
			deletes = append(deletes, t)
		}
	}
	return writes, deletes, nil
}

// RemoveMember deletes all four role relations and each direct user grant.
// Userset grants are never named here.
func (Catalog) RemoveMember(tenantID, userID string, grants []domain.Grant) []usecase.Tuple {
	var deletes []usecase.Tuple
	for _, r := range domain.AllRoles {
		deletes = append(deletes, usecase.Tuple{User: userRef(userID), Relation: r, Object: obj("tenant", tenantID)})
	}
	for _, g := range grants {
		if g.SubjectType != domain.SubjectUser || g.UserID == nil || *g.UserID != userID {
			continue
		}
		deletes = append(deletes, usecase.Tuple{User: userRef(userID), Relation: g.Relation, Object: obj(g.ObjectType, g.ObjectID)})
	}
	return deletes
}

// CreateContainer links the container to its tenant and owner, then either to
// its parent or (top level) to the tenant as `root`, which anchors the
// "every ancestor is public" chain. A public container also gets `public`.
func (Catalog) CreateContainer(kind domain.Kind, tenantID, id, ownerID string, parentID *string, isPrivate bool) []usecase.Tuple {
	o := obj(kind.Container, id)
	ts := []usecase.Tuple{
		{User: obj("tenant", tenantID), Relation: "tenant", Object: o},
		{User: userRef(ownerID), Relation: domain.RelOwner, Object: o},
	}
	if parentID != nil {
		ts = append(ts, usecase.Tuple{User: obj(kind.Container, *parentID), Relation: "parent", Object: o})
	} else {
		ts = append(ts, usecase.Tuple{User: obj("tenant", tenantID), Relation: "root", Object: o})
	}
	if !isPrivate {
		ts = append(ts, publicTuple(kind, tenantID, id))
	}
	return ts
}

func publicTuple(kind domain.Kind, tenantID, id string) usecase.Tuple {
	return usecase.Tuple{User: obj("tenant", tenantID), Relation: "public", Object: obj(kind.Container, id)}
}

func (Catalog) SetVisibility(kind domain.Kind, tenantID, id string, isPrivate bool) (writes, deletes []usecase.Tuple) {
	t := publicTuple(kind, tenantID, id)
	if isPrivate {
		return nil, []usecase.Tuple{t}
	}
	return []usecase.Tuple{t}, nil
}

func (Catalog) CreateItem(kind domain.Kind, tenantID, id, containerID, ownerID string) []usecase.Tuple {
	o := obj(kind.Item, id)
	return []usecase.Tuple{
		{User: obj("tenant", tenantID), Relation: "tenant", Object: o},
		{User: obj(kind.Container, containerID), Relation: kind.ContainerRef, Object: o},
		{User: userRef(ownerID), Relation: domain.RelOwner, Object: o},
	}
}

// Grant returns the tuple for a resource grant. Owner grants are only valid
// for a user subject; share validation of owner happens in the usecase.
func (Catalog) Grant(g domain.Grant) (usecase.Tuple, error) {
	switch g.Relation {
	case domain.RelOwner, domain.RelEditor, domain.RelViewer:
	default:
		return usecase.Tuple{}, domain.Invalid("relation must be viewer or editor")
	}
	o := obj(g.ObjectType, g.ObjectID)
	if g.SubjectType == domain.SubjectUser {
		if g.UserID == nil || *g.UserID == "" {
			return usecase.Tuple{}, domain.Invalid("user_id is required for subject user")
		}
		return usecase.Tuple{User: userRef(*g.UserID), Relation: g.Relation, Object: o}, nil
	}
	rel, ok := domain.SubjectRelation[g.SubjectType]
	if !ok {
		return usecase.Tuple{}, domain.Invalid("unknown subject %q", g.SubjectType)
	}
	if g.Relation == domain.RelOwner {
		return usecase.Tuple{}, domain.Invalid("owner cannot be a userset")
	}
	if g.SubjectType == domain.SubjectExternalUsers && g.Relation == domain.RelEditor {
		return usecase.Tuple{}, domain.ErrCannotUpload
	}
	return usecase.Tuple{User: userset(g.TenantID, rel), Relation: g.Relation, Object: o}, nil
}
