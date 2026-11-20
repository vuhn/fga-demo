package output

import (
	"time"
)

// Project is the usecase output copy of a project.
type Project struct {
	ID        string
	TenantID  string
	Name      string
	CreatedBy string
	CreatedAt time.Time
}

// ProjectCreator is the result of ProjectCreator.Create.
type ProjectCreator struct {
	Project
}

// ProjectGetter is the result of ProjectGetter.Get.
type ProjectGetter struct {
	Visible[Project]
}

// ProjectLister is the result of ProjectLister.List.
type ProjectLister struct {
	Items []Visible[Project]
}

// ProjectMember is the usecase output copy of a projectMember.
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

// ProjectMemberAdder is the result of ProjectMemberAdder.Add.
type ProjectMemberAdder struct {
	ProjectMember
}

// ProjectMemberCandidateLister is the result of ProjectMemberCandidateLister.ListCandidates.
type ProjectMemberCandidateLister struct {
	Items []Membership
}

// ProjectMemberLister is the result of ProjectMemberLister.List.
type ProjectMemberLister struct {
	Items []ProjectMember
}

// ProjectMemberRoleChanger is the result of ProjectMemberRoleChanger.ChangeRole.
type ProjectMemberRoleChanger struct {
	ProjectMember
}

// ProjectRenamer is the result of ProjectRenamer.Rename.
type ProjectRenamer struct {
	Project
}

// ProjectRole is the usecase output copy of a projectRole.
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

// ProjectRoleCreator is the result of ProjectRoleCreator.Create.
type ProjectRoleCreator struct {
	ProjectRole
}

// ProjectRoleLister is the result of ProjectRoleLister.List.
type ProjectRoleLister struct {
	Items []ProjectRole
}

// ProjectRolePermissionSetter is the result of ProjectRolePermissionSetter.SetPermissions.
type ProjectRolePermissionSetter struct {
	ProjectRole
}
