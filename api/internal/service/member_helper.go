package service

import (
	"context"
	"fmt"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
)

// keepOneInternalAdmin is a stored invariant, not an authorization check.
func keepOneInternalAdmin(ctx context.Context, members gateway.MembershipGateway, m entity.Membership) error {
	if m.Role != entity.RoleInternalAdmin {
		return nil
	}
	n, err := members.CountInternalAdmins(ctx, m.TenantID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return fmt.Errorf("%w: tenant must keep at least one internal admin", entity.ErrConflict)
	}
	return nil
}
