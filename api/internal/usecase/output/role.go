package output

import (
	"time"
)

// Role is the usecase output copy of a role.
type Role struct {
	ID          string
	TenantID    string
	Name        string
	Affiliation string
	Permissions []string
	Assignees   []RoleAssignee
	CreatedAt   time.Time
}

// RoleAssignee is the usecase output copy of a roleAssignee.
type RoleAssignee struct {
	UserID string
	Email  string
	Name   string
}

// RoleCreator is the result of RoleCreator.Create.
type RoleCreator struct {
	Role
}

// RoleGetter is the result of RoleGetter.Get.
type RoleGetter struct {
	Role
}

// RoleLister is the result of RoleLister.List.
type RoleLister struct {
	Items []Role
}

// RolePermissionSetter is the result of RolePermissionSetter.SetPermissions.
type RolePermissionSetter struct {
	Role
}

// RoleUpdater is the result of RoleUpdater.UpdateName.
type RoleUpdater struct {
	Role
}
