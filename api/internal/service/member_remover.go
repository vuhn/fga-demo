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
	roles     gateway.RoleGateway
	roleRels  gateway.RoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewMemberRemover(members gateway.MembershipGateway, relations gateway.MembershipRelationGateway, roles gateway.RoleGateway, roleRels gateway.RoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.MemberRemover {
	return &memberRemover{members: members, relations: relations, roles: roles, roleRels: roleRels, authz: authz, guard: guard}
}

// Remove deletes the built-in role tuples, custom-role assignee tuples, and
// every direct grant for the user in this tenant. Group usersets stay; the
// person simply drops out of them.
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
	roleIDs, err := s.roles.ListRoleIDsForUser(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	for _, id := range roleIDs {
		deletes = append(deletes, s.roleRels.Assignee(id, userID))
	}

	// Restore set: exactly the tuples that existed.
	var restore []entity.Tuple
	if m.CustomRoleID == "" {
		var err error
		restore, _, err = s.relations.AssignRole(tenantID, userID, m.Affiliation, m.Role)
		if err != nil {
			return err
		}
	}
	grantTuples, err := s.relations.Grants(grants)
	if err != nil {
		return err
	}
	restore = append(restore, grantTuples...)
	for _, id := range roleIDs {
		restore = append(restore, s.roleRels.Assignee(id, userID))
	}

	return s.guard.Revoke(ctx, deletes, restore, func(ctx context.Context) error {
		return s.members.Delete(ctx, tenantID, userID)
	})
}
