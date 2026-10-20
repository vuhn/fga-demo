package entity

import (
	"strings"
	"time"
)

const ObjectTypeRole = "role"

// Fixed permission catalog. The OpenFGA model cannot grow a new relation per
// tenant, so a custom role is an object and each permission is one grant_*
// relation on the tenant. Higher permissions imply lower ones in the model;
// callers write only the permission they assigned.
const (
	PermView             = "view"
	PermEdit             = "edit"
	PermManage           = "manage"
	PermChangeVisibility = "change_visibility"
	PermContribute       = "contribute"
	PermCreatePrivate    = "create_private"
	PermManageMembers    = "manage_members"
	PermViewProducts     = "view_products"
)

// CustomPermissions is the catalog order used in API responses.
var CustomPermissions = []string{
	PermView,
	PermEdit,
	PermManage,
	PermChangeVisibility,
	PermContribute,
	PermCreatePrivate,
	PermManageMembers,
	PermViewProducts,
}

// GrantRelation is the tenant relation a permission writes.
func GrantRelation(permission string) (string, bool) {
	switch permission {
	case PermView, PermEdit, PermManage, PermChangeVisibility, PermContribute, PermCreatePrivate, PermManageMembers, PermViewProducts:
		return "grant_" + permission, true
	default:
		return "", false
	}
}

// NormalizePermissions dedupes and returns catalog order. Unknown names are
// a validation error. An empty list is valid: the role grants nothing.
func NormalizePermissions(in []string) ([]string, error) {
	seen := map[string]bool{}
	for _, p := range in {
		if _, ok := GrantRelation(p); !ok {
			return nil, Invalid("unknown permission %q", p)
		}
		seen[p] = true
	}
	out := make([]string, 0, len(seen))
	for _, p := range CustomPermissions {
		if seen[p] {
			out = append(out, p)
		}
	}
	return out, nil
}

// CleanRoleName trims a custom-role name. The four built-in role ids are reserved.
func CleanRoleName(s string) (string, error) {
	s, err := CleanName("name", s)
	if err != nil {
		return "", err
	}
	for _, r := range AllRoles {
		if strings.EqualFold(s, r) {
			return "", Invalid("name is reserved for a built-in role")
		}
	}
	return s, nil
}

// Role is a tenant-scoped custom role of one affiliation. A member holds it
// the same way they hold a built-in role: one role, and it never crosses
// affiliation. Permissions and assignees are display labels for tuples.
type Role struct {
	ID          string
	TenantID    string
	Name        string
	Affiliation string
	Permissions []string
	Assignees   []RoleAssignee
	CreatedAt   time.Time
}

// RoleAssignee is a member who holds the role.
type RoleAssignee struct {
	UserID string
	Email  string
	Name   string
}
