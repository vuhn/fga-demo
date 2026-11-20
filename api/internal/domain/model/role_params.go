package model

// RemoveRoleParams describes a custom role whose tuples are removed: its
// anchors, its grants and its assignees.
type RemoveRoleParams struct {
	TenantID    string
	RoleID      string
	Affiliation string
	Permissions []string
	UserIDs     []string
}
