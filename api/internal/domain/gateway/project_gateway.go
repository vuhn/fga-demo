package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// ProjectGateway is the projects display record.
type ProjectGateway interface {
	Create(ctx context.Context, p entity.Project) error
	Get(ctx context.Context, id string) (entity.Project, error)
	// ListByIDs loads the given ids, restricted to the tenant.
	ListByIDs(ctx context.Context, tenantID string, ids []string) ([]entity.Project, error)
	Rename(ctx context.Context, id, name string) (entity.Project, error)
	// Delete is the compensation for a failed tuple write; it fails while the project still has content.
	Delete(ctx context.Context, id string) error
}

// ProjectRelationGateway builds the project tuples. It is pure.
type ProjectRelationGateway interface {
	// Created links the project to its tenant.
	Created(p entity.Project) []entity.Tuple
}

// ProjectRoleGateway is the display record for project roles.
type ProjectRoleGateway interface {
	List(ctx context.Context, projectID string) ([]entity.ProjectRole, error)
	Get(ctx context.Context, projectID, roleID string) (entity.ProjectRole, error)
	Insert(ctx context.Context, role entity.ProjectRole) error
	Delete(ctx context.Context, projectID, roleID string) error
	InsertPermissions(ctx context.Context, roleID string, permissions []string) error
	DeletePermissions(ctx context.Context, roleID string, permissions []string) error
}

// ProjectRoleRelationGateway builds project-role tuples. It is pure.
type ProjectRoleRelationGateway interface {
	Created(projectID, roleID, affiliation string) ([]entity.Tuple, error)
	Permissions(projectID, roleID string, permissions []string) ([]entity.Tuple, error)
	Assignee(roleID, userID string) entity.Tuple
	// Removal is every tuple the role owns: anchors and grants.
	Removal(projectID, roleID, affiliation string, permissions []string) ([]entity.Tuple, error)
}

// ProjectMemberGateway is the project_members display record. The row
// references tenant_members, so only tenant members can be project members.
type ProjectMemberGateway interface {
	Get(ctx context.Context, projectID, userID string) (entity.ProjectMember, error)
	List(ctx context.Context, projectID string) ([]entity.ProjectMember, error)
	// ListForUser returns the user's memberships in every project of the tenant.
	ListForUser(ctx context.Context, tenantID, userID string) ([]entity.ProjectMember, error)
	Insert(ctx context.Context, m entity.ProjectMember) error
	UpdateRole(ctx context.Context, projectID, userID, role, projectRoleID string) error
	Delete(ctx context.Context, projectID, userID string) error
}

// ProjectMemberRelationGateway builds project role tuples for a member. It is pure.
type ProjectMemberRelationGateway interface {
	// AssignRole validates the pair and returns the tuple for the role plus
	// deletes for the other three built-in roles.
	AssignRole(projectID, userID, affiliation, role string) (writes, deletes []entity.Tuple, err error)
	// ClearRoles deletes all four built-in role tuples.
	ClearRoles(projectID, userID string) []entity.Tuple
}
