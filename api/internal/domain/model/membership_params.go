package model

// UpdateMemberRoleParams changes a member's role in a tenant; CustomRoleID is
// set when the role is a custom one.
type UpdateMemberRoleParams struct {
	TenantID     string
	UserID       string
	Role         string
	CustomRoleID string
}

// AssignMemberRoleParams names the role a member gets in a tenant, for the
// tuples that record it.
type AssignMemberRoleParams struct {
	TenantID    string
	UserID      string
	Affiliation string
	Role        string
}
