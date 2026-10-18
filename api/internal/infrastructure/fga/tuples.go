package fga

import "github.com/vuhn/fga-demo/api/internal/domain/entity"

// Tuple formatting lives only in this package. Every builder is pure.

func obj(typ, id string) string           { return typ + ":" + id }
func userRef(id string) string            { return "user:" + id }
func userset(tenantID, rel string) string { return "tenant:" + tenantID + "#" + rel }

const wildcard = "user:*"

// PlatformAdmin is written only by the seed.
func PlatformAdmin(userID string) entity.Tuple {
	return entity.Tuple{User: userRef(userID), Relation: "system_admin", Object: obj(entity.ObjectTypePlatform, entity.PlatformID)}
}

// grantTuple returns the tuple for a resource grant on objectType. Owner
// grants are only valid for a user subject; share validation of owner
// happens in the service.
func grantTuple(objectType string, g entity.Grant) (entity.Tuple, error) {
	switch g.Relation {
	case entity.RelOwner, entity.RelEditor, entity.RelViewer:
	default:
		return entity.Tuple{}, entity.Invalid("relation must be viewer or editor")
	}
	o := obj(objectType, g.ObjectID)
	if g.SubjectType == entity.SubjectUser {
		if g.UserID == nil || *g.UserID == "" {
			return entity.Tuple{}, entity.Invalid("user_id is required for subject user")
		}
		return entity.Tuple{User: userRef(*g.UserID), Relation: g.Relation, Object: o}, nil
	}
	rel, ok := entity.SubjectRelation[g.SubjectType]
	if !ok {
		return entity.Tuple{}, entity.Invalid("unknown subject %q", g.SubjectType)
	}
	if g.Relation == entity.RelOwner {
		return entity.Tuple{}, entity.Invalid("owner cannot be a userset")
	}
	if g.SubjectType == entity.SubjectExternalUsers && g.Relation == entity.RelEditor {
		return entity.Tuple{}, entity.ErrCannotUpload
	}
	return entity.Tuple{User: userset(g.TenantID, rel), Relation: g.Relation, Object: o}, nil
}
