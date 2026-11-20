package input

// ProjectCreator is the input of ProjectCreator.Create.
type ProjectCreator struct {
	Actor    string
	TenantID string
	Name     string
}

// ProjectGetter is the input of ProjectGetter.Get.
type ProjectGetter struct {
	Actor    string
	TenantID string
	ID       string
}

// ProjectLister is the input of ProjectLister.List.
type ProjectLister struct {
	Actor    string
	TenantID string
}

// ProjectMemberAdder is the input of ProjectMemberAdder.Add.
type ProjectMemberAdder struct {
	Actor     string
	TenantID  string
	ProjectID string
	UserID    string
	Role      string
}

// ProjectMemberCandidateLister is the input of ProjectMemberCandidateLister.ListCandidates.
type ProjectMemberCandidateLister struct {
	Actor     string
	TenantID  string
	ProjectID string
}

// ProjectMemberLister is the input of ProjectMemberLister.List.
type ProjectMemberLister struct {
	Actor     string
	TenantID  string
	ProjectID string
}

// ProjectMemberRemover is the input of ProjectMemberRemover.Remove.
type ProjectMemberRemover struct {
	Actor     string
	TenantID  string
	ProjectID string
	UserID    string
}

// ProjectMemberRoleChanger is the input of ProjectMemberRoleChanger.ChangeRole.
type ProjectMemberRoleChanger struct {
	Actor     string
	TenantID  string
	ProjectID string
	UserID    string
	Role      string
}

// ProjectRenamer is the input of ProjectRenamer.Rename.
type ProjectRenamer struct {
	Actor    string
	TenantID string
	ID       string
	Name     string
}

// ProjectRoleCreator is the input of ProjectRoleCreator.Create.
type ProjectRoleCreator struct {
	Actor       string
	TenantID    string
	ProjectID   string
	Name        string
	Affiliation string
}

// ProjectRoleDeleter is the input of ProjectRoleDeleter.Delete.
type ProjectRoleDeleter struct {
	Actor     string
	TenantID  string
	ProjectID string
	RoleID    string
}

// ProjectRoleLister is the input of ProjectRoleLister.List.
type ProjectRoleLister struct {
	Actor     string
	TenantID  string
	ProjectID string
}

// ProjectRolePermissionSetter is the input of ProjectRolePermissionSetter.SetPermissions.
type ProjectRolePermissionSetter struct {
	Actor       string
	TenantID    string
	ProjectID   string
	RoleID      string
	Permissions []string
}
