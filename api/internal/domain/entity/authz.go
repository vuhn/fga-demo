package entity

import "sort"

// Tuple is a fully formatted OpenFGA relationship tuple. Only the fga
// infrastructure builds them; everything else passes them through.
type Tuple struct {
	User     string // "user:<uuid>", "user:*", or "tenant:<uuid>#<relation>"
	Relation string
	Object   string // "<type>:<id>"
}

type CheckRequest struct {
	UserID     string
	Relation   string
	ObjectType string
	ObjectID   string
}

const ObjectTypePlatform = "platform"
const ObjectTypeTenant = "tenant"

// PermissionRelations is the permission relations (never raw grant relations)
// per object type. It is the inspector whitelist and the set reported in
// `permissions` objects.
var PermissionRelations = map[string][]string{
	ObjectTypePlatform: {"can_manage_platform"},
	ObjectTypeTenant: sorted("can_manage_members", "can_view_products", "can_contribute", "can_read_tenant",
		"can_create_folder", "can_create_album", "can_create_private_folder", "can_create_private_album",
		"documents_enabled", "photos_enabled"),
	ObjectTypeFolder:   sorted("can_view", "can_edit", "can_manage", "can_delete", "can_share", "can_change_visibility", "can_create_document"),
	ObjectTypeDocument: sorted("can_view", "can_edit", "can_manage", "can_delete", "can_share"),
	ObjectTypeAlbum:    sorted("can_view", "can_edit", "can_manage", "can_delete", "can_share", "can_change_visibility", "can_create_photo"),
	ObjectTypePhoto:    sorted("can_view", "can_edit", "can_manage", "can_delete", "can_share"),
}

// IsPermissionRelation reports whether relation is whitelisted on objectType.
func IsPermissionRelation(objectType, relation string) bool {
	for _, r := range PermissionRelations[objectType] {
		if r == relation {
			return true
		}
	}
	return false
}

func sorted(xs ...string) []string {
	sort.Strings(xs)
	return xs
}

// Perms is the caller's own allow/deny for each permission relation on one
// object. It drives what the UI shows; the API still checks on every call.
type Perms map[string]bool

// Visible pairs an object the caller can view with the caller's permissions on it.
type Visible[T any] struct {
	Value T
	Perms Perms
}
