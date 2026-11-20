package input

// RoleCreator is the input of RoleCreator.Create.
type RoleCreator struct {
	Actor       string
	TenantID    string
	Name        string
	Affiliation string
}

// RoleDeleter is the input of RoleDeleter.Delete.
type RoleDeleter struct {
	Actor    string
	TenantID string
	RoleID   string
}

// RoleGetter is the input of RoleGetter.Get.
type RoleGetter struct {
	Actor    string
	TenantID string
	RoleID   string
}

// RoleLister is the input of RoleLister.List.
type RoleLister struct {
	Actor    string
	TenantID string
}

// RolePermissionSetter is the input of RolePermissionSetter.SetPermissions.
type RolePermissionSetter struct {
	Actor       string
	TenantID    string
	RoleID      string
	Permissions []string
}

// RoleUpdater is the input of RoleUpdater.UpdateName.
type RoleUpdater struct {
	Actor    string
	TenantID string
	RoleID   string
	Name     string
}
