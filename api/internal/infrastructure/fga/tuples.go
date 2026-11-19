package fga

import "github.com/vuhn/fga-demo/api/internal/domain/model"

// Tuple formatting lives only in this package. Every builder is pure.

func obj(typ, id string) string           { return typ + ":" + id }
func userRef(id string) string            { return "user:" + id }
func userset(tenantID, rel string) string { return "tenant:" + tenantID + "#" + rel }

const wildcard = "user:*"

// PlatformAdmin is written only by the seed.
func PlatformAdmin(userID string) model.Tuple {
	return model.Tuple{User: userRef(userID), Relation: "system_admin", Object: obj(model.ObjectTypePlatform, model.PlatformID)}
}

// grantTuple returns the tuple for a resource grant on objectType. Owner
// grants are only valid for a user subject; share validation of owner
// happens in the service.
func grantTuple(objectType string, g model.Grant) (model.Tuple, error) {
	switch g.Relation {
	case model.RelOwner, model.RelEditor, model.RelViewer:
	default:
		return model.Tuple{}, model.Invalid("relation must be viewer or editor")
	}
	o := obj(objectType, g.ObjectID)
	if g.SubjectType == model.SubjectUser {
		if g.UserID == nil || *g.UserID == "" {
			return model.Tuple{}, model.Invalid("user_id is required for subject user")
		}
		return model.Tuple{User: userRef(*g.UserID), Relation: g.Relation, Object: o}, nil
	}
	rel, ok := model.SubjectRelation[g.SubjectType]
	if !ok {
		return model.Tuple{}, model.Invalid("unknown subject %q", g.SubjectType)
	}
	if g.Relation == model.RelOwner {
		return model.Tuple{}, model.Invalid("owner cannot be a userset")
	}
	if g.SubjectType == model.SubjectExternalUsers && g.Relation == model.RelEditor {
		return model.Tuple{}, model.ErrCannotUpload
	}
	return model.Tuple{User: userset(g.TenantID, rel), Relation: g.Relation, Object: o}, nil
}
