package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/domain/model"
)

// keepOneInternalAdmin is a stored invariant, not an authorization check.
func keepOneInternalAdmin(ctx context.Context, members gateway.MembershipGateway, m model.Membership) error {
	if m.Role != model.RoleInternalAdmin {
		return nil
	}
	n, err := members.CountInternalAdmins(ctx, m.TenantID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return fmt.Errorf("%w: tenant must keep at least one internal admin", model.ErrConflict)
	}
	return nil
}
