package entity

const (
	AffiliationInternal = "internal"
	AffiliationExternal = "external"

	RoleInternalAdmin = "internal_admin"
	RoleInternalUser  = "internal_user"
	RoleExternalAdmin = "external_admin"
	RoleExternalUser  = "external_user"
)

// Roles are also the OpenFGA tenant relation names.
var AllRoles = []string{RoleInternalAdmin, RoleInternalUser, RoleExternalAdmin, RoleExternalUser}

var rolesByAffiliation = map[string][]string{
	AffiliationInternal: {RoleInternalAdmin, RoleInternalUser},
	AffiliationExternal: {RoleExternalAdmin, RoleExternalUser},
}

// CheckAffiliationRole validates an affiliation and role pair against the
// legal matrix.
func CheckAffiliationRole(affiliation, role string) error {
	roles, ok := rolesByAffiliation[affiliation]
	if !ok {
		return Invalid("unknown affiliation %q", affiliation)
	}
	known := false
	for _, r := range AllRoles {
		if r == role {
			known = true
		}
	}
	if !known {
		return Invalid("unknown role %q", role)
	}
	for _, r := range roles {
		if r == role {
			return nil
		}
	}
	return ErrAffiliationRoleMismatch
}

// Membership is a person's display labels in a tenant. Role and Affiliation
// are never used to permit anything; OpenFGA decides.
type Membership struct {
	TenantID    string
	UserID      string
	Affiliation string
	Role        string
	Email       string
	Name        string
}
