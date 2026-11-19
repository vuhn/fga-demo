package gateway

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

type MembershipGateway interface {
	Get(ctx context.Context, tenantID, userID string) (model.Membership, error)
	// List filters by affiliation when it is not empty.
	List(ctx context.Context, tenantID, affiliation string) ([]model.Membership, error)
	ListForUser(ctx context.Context, userID string) ([]model.Membership, error)
	Insert(ctx context.Context, m model.Membership) error
	// UpdateRole sets the built-in role, or a custom role id with customRoleID set.
	UpdateRole(ctx context.Context, tenantID, userID, role, customRoleID string) error
	CountInternalAdmins(ctx context.Context, tenantID string) (int, error)
	// ListUserGrants returns the user's direct grants on any content in the tenant.
	ListUserGrants(ctx context.Context, tenantID, userID string) ([]model.Grant, error)
	// Delete removes the membership and the user's grants in that tenant in
	// one transaction.
	Delete(ctx context.Context, tenantID, userID string) error
}

// MembershipRelationGateway builds the role tuples. It is pure.
type MembershipRelationGateway interface {
	// AssignRole validates the pair and returns the tuple for the role plus
	// deletes for the other three, so a person holds exactly one direct role.
	AssignRole(tenantID, userID, affiliation, role string) (writes, deletes []model.Tuple, err error)
	// Removal deletes every role relation and the user's direct grants.
	// Userset grants are never named.
	Removal(tenantID, userID string, grants []model.Grant) []model.Tuple
	// Grants returns the tuples of existing grants, for restoring them.
	Grants(grants []model.Grant) ([]model.Tuple, error)
	// ClearRoles deletes all four built-in role tuples for the user.
	ClearRoles(tenantID, userID string) []model.Tuple
}
