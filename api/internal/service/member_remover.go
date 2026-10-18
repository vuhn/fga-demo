package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type memberRemover struct {
	members   gateway.MembershipGateway
	relations gateway.MembershipRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewMemberRemover(members gateway.MembershipGateway, relations gateway.MembershipRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.MemberRemover {
	return &memberRemover{members: members, relations: relations, authz: authz, guard: guard}
}

// Remove deletes the role tuples and every direct grant for the user in this
// tenant. Userset tuples stay; the person simply drops out of them.
func (s *memberRemover) Remove(ctx context.Context, actor, tenantID, userID string) error {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return err
	}
	m, err := s.members.Get(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if err := keepOneInternalAdmin(ctx, s.members, m); err != nil {
		return err
	}
	grants, err := s.members.ListUserGrants(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	deletes := s.relations.Removal(tenantID, userID, grants)

	// Restore set: exactly the tuples that existed.
	restore, _, err := s.relations.AssignRole(tenantID, userID, m.Affiliation, m.Role)
	if err != nil {
		return err
	}
	grantTuples, err := s.relations.Grants(grants)
	if err != nil {
		return err
	}
	restore = append(restore, grantTuples...)

	return s.guard.Revoke(ctx, deletes, restore, func(ctx context.Context) error {
		return s.members.Delete(ctx, tenantID, userID)
	})
}
