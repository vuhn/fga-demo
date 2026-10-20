package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
)

// RoleGateway is the display record for custom roles.
type RoleGateway interface {
	List(ctx context.Context, tenantID string) ([]entity.Role, error)
	Get(ctx context.Context, tenantID, roleID string) (entity.Role, error)
	Insert(ctx context.Context, role entity.Role) error
	UpdateName(ctx context.Context, tenantID, roleID, name string) error
	Delete(ctx context.Context, tenantID, roleID string) error
	InsertPermissions(ctx context.Context, roleID string, permissions []string) error
	DeletePermissions(ctx context.Context, roleID string, permissions []string) error
	// ListRoleIDsForUser returns custom roles this member holds in the tenant.
	ListRoleIDsForUser(ctx context.Context, tenantID, userID string) ([]string, error)
}

// RoleRelationGateway builds custom-role tuples. It is pure.
type RoleRelationGateway interface {
	// Created anchors the role. affiliation is internal or external.
	Created(tenantID, roleID, affiliation string) ([]entity.Tuple, error)
	Permissions(tenantID, roleID string, permissions []string) ([]entity.Tuple, error)
	Assignee(roleID, userID string) entity.Tuple
	// Removal is every tuple the role owns: anchors, grants, and assignees.
	Removal(tenantID, roleID, affiliation string, permissions, userIDs []string) ([]entity.Tuple, error)
}
