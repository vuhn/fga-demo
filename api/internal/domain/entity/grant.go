package entity

import "time"

// Grant subject types. Internal members already see all content, so group
// shares exist only for external members.
const (
	SubjectUser           = "user"
	SubjectExternalUsers  = "external_users"
	SubjectExternalAdmins = "external_admins"
)

// SubjectRelation maps a userset subject type to the tenant relation it names.
var SubjectRelation = map[string]string{
	SubjectExternalUsers:  RoleExternalUser,
	SubjectExternalAdmins: RoleExternalAdmin,
}

const (
	RelOwner  = "owner"
	RelEditor = "editor"
	RelViewer = "viewer"
)

// Grant is a resource_grants row: who holds owner, editor, or viewer on one
// content object.
type Grant struct {
	ID          string
	TenantID    string
	ObjectType  string
	ObjectID    string
	Relation    string
	SubjectType string
	UserID      *string
	UserName    string // display only, filled on list
	CreatedAt   time.Time
}

// OwnerGrant is the grant recorded for the creator of a content object.
func OwnerGrant(id, tenantID, objectType, objectID, userID string) Grant {
	return Grant{ID: id, TenantID: tenantID, ObjectType: objectType, ObjectID: objectID,
		Relation: RelOwner, SubjectType: SubjectUser, UserID: &userID}
}

// SameSubject reports whether two grants name the same subject.
func (g Grant) SameSubject(o Grant) bool {
	if g.SubjectType != o.SubjectType {
		return false
	}
	if g.SubjectType != SubjectUser {
		return true
	}
	return g.UserID != nil && o.UserID != nil && *g.UserID == *o.UserID
}

// ShareInput is a share or revoke request on one content object.
type ShareInput struct {
	Relation string
	Subject  string
	UserID   string
}
