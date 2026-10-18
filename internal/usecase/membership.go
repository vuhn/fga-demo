package usecase

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/internal/domain"
)

// ListMembers is tenant administration: only someone who can manage members
// (the internal admin) or the platform may read the roster. The affiliation
// filter is a display query after the Check.
func (a *App) ListMembers(ctx context.Context, actor, tenantID, affiliation string) ([]domain.Membership, error) {
	switch affiliation {
	case "", domain.AffiliationInternal, domain.AffiliationExternal:
	default:
		return nil, domain.Invalid("affiliation must be internal or external")
	}
	if err := a.platformOr(ctx, actor, "can_manage_members", tenantID, domain.ErrForbidden); err != nil {
		return nil, err
	}
	if _, err := a.Store.GetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	return a.Store.ListMembers(ctx, tenantID, affiliation)
}

func (a *App) AddMember(ctx context.Context, actor, tenantID, userID, affiliation, role string) (domain.Membership, error) {
	if err := requireUUID("user_id", userID); err != nil {
		return domain.Membership{}, err
	}
	if err := a.platformOr(ctx, actor, "can_manage_members", tenantID, domain.ErrForbidden); err != nil {
		return domain.Membership{}, err
	}
	// Validates the affiliation/role pair before anything is written.
	writes, deletes, err := a.Tuples.AssignRole(tenantID, userID, affiliation, role)
	if err != nil {
		return domain.Membership{}, err
	}
	if _, err := a.Store.GetTenant(ctx, tenantID); err != nil {
		return domain.Membership{}, err
	}
	if _, err := a.Store.GetUser(ctx, userID); err != nil {
		return domain.Membership{}, err
	}
	m := domain.Membership{TenantID: tenantID, UserID: userID, Affiliation: affiliation, Role: role}
	if err := a.Store.InsertMembership(ctx, m); err != nil {
		return domain.Membership{}, err
	}
	err = a.grant(ctx, writes, deletes, func(ctx context.Context) error {
		return a.Store.RemoveMember(ctx, tenantID, userID)
	})
	if err != nil {
		return domain.Membership{}, err
	}
	return a.Store.GetMembership(ctx, tenantID, userID)
}

// ChangeRole keeps the affiliation. A role from the other affiliation is a
// mismatch, not a move.
func (a *App) ChangeRole(ctx context.Context, actor, tenantID, userID, role string) (domain.Membership, error) {
	if err := a.platformOr(ctx, actor, "can_manage_members", tenantID, domain.ErrForbidden); err != nil {
		return domain.Membership{}, err
	}
	m, err := a.Store.GetMembership(ctx, tenantID, userID)
	if err != nil {
		return domain.Membership{}, err
	}
	writes, deletes, err := a.Tuples.AssignRole(tenantID, userID, m.Affiliation, role)
	if err != nil {
		return domain.Membership{}, err
	}
	if m.Role == role {
		return m, nil
	}
	if err := a.keepOneInternalAdmin(ctx, m); err != nil {
		return domain.Membership{}, err
	}
	if err := a.Store.UpdateRole(ctx, tenantID, userID, role); err != nil {
		return domain.Membership{}, err
	}
	err = a.grant(ctx, writes, deletes, func(ctx context.Context) error {
		return a.Store.UpdateRole(ctx, tenantID, userID, m.Role)
	})
	if err != nil {
		return domain.Membership{}, err
	}
	return a.Store.GetMembership(ctx, tenantID, userID)
}

// RemoveMember deletes the role tuples and every direct grant for the user in
// this tenant. Userset tuples stay; the person simply drops out of them.
func (a *App) RemoveMember(ctx context.Context, actor, tenantID, userID string) error {
	if err := a.platformOr(ctx, actor, "can_manage_members", tenantID, domain.ErrForbidden); err != nil {
		return err
	}
	m, err := a.Store.GetMembership(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if err := a.keepOneInternalAdmin(ctx, m); err != nil {
		return err
	}
	grants, err := a.Store.GrantsForUser(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	deletes := a.Tuples.RemoveMember(tenantID, userID, grants)

	// Restore set: exactly the tuples that existed.
	restore, _, err := a.Tuples.AssignRole(tenantID, userID, m.Affiliation, m.Role)
	if err != nil {
		return err
	}
	for _, g := range grants {
		t, err := a.Tuples.Grant(g)
		if err != nil {
			return err
		}
		restore = append(restore, t)
	}
	return a.revoke(ctx, deletes, restore, func(ctx context.Context) error {
		return a.Store.RemoveMember(ctx, tenantID, userID)
	})
}

// keepOneInternalAdmin is a stored invariant, not an authorization check.
func (a *App) keepOneInternalAdmin(ctx context.Context, m domain.Membership) error {
	if m.Role != domain.RoleInternalAdmin {
		return nil
	}
	n, err := a.Store.CountInternalAdmins(ctx, m.TenantID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return fmt.Errorf("%w: tenant must keep at least one internal admin", domain.ErrConflict)
	}
	return nil
}
