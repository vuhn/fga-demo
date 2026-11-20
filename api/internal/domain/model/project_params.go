package model

// UpdateProjectMemberRoleParams changes a member's role in a project;
// ProjectRoleID is set when the role is a custom project role.
type UpdateProjectMemberRoleParams struct {
	ProjectID     string
	UserID        string
	Role          string
	ProjectRoleID string
}

// AssignProjectMemberRoleParams names the role a member gets in a project,
// for the tuples that record it.
type AssignProjectMemberRoleParams struct {
	ProjectID   string
	UserID      string
	Affiliation string
	Role        string
}

// RemoveProjectRoleParams describes a project role whose tuples are removed.
type RemoveProjectRoleParams struct {
	ProjectID   string
	RoleID      string
	Affiliation string
	Permissions []string
}
