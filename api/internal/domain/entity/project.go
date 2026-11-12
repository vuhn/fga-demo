package entity

import "time"

// Project is a unit of work inside a tenant. Folders, albums, documents,
// and photos belong to one project.
type Project struct {
	ID        string
	TenantID  string
	Name      string
	CreatedBy string
	CreatedAt time.Time
}

// ProjectRole is a project-scoped custom role of one affiliation. It has the
// same permission catalog as a tenant Role; its grants apply inside its
// project only.
type ProjectRole struct {
	ID          string
	TenantID    string
	ProjectID   string
	Name        string
	Affiliation string
	Permissions []string
	Assignees   []RoleAssignee
	CreatedAt   time.Time
}

// ProjectMember is a tenant member's display labels in one project. Role is
// a built-in role name or a project role id. Nothing branches on it.
type ProjectMember struct {
	ProjectID     string
	TenantID      string
	UserID        string
	Affiliation   string
	Role          string
	RoleName      string
	ProjectRoleID string
	Email         string
	Name          string
	CompanyID     string
	CompanyName   string
}
