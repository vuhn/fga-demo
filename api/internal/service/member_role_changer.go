package service

import (
	"context"

	"github.com/vuhn/fga-demo/api/internal/domain/entity"
	"github.com/vuhn/fga-demo/api/internal/domain/gateway"
	"github.com/vuhn/fga-demo/api/internal/usecase"
	"github.com/vuhn/fga-demo/api/internal/usecase/helper"
)

type memberRoleChanger struct {
	members   gateway.MembershipGateway
	roles     gateway.RoleGateway
	relations gateway.MembershipRelationGateway
	roleRels  gateway.RoleRelationGateway
	authz     usecase.Authorizer
	guard     helper.WriteGuard
}

func NewMemberRoleChanger(members gateway.MembershipGateway, roles gateway.RoleGateway, relations gateway.MembershipRelationGateway, roleRels gateway.RoleRelationGateway, authz usecase.Authorizer, guard helper.WriteGuard) usecase.MemberRoleChanger {
	return &memberRoleChanger{members: members, roles: roles, relations: relations, roleRels: roleRels, authz: authz, guard: guard}
}

// ChangeRole keeps the affiliation. A role from the other affiliation is a
// mismatch, not a move.
func (s *memberRoleChanger) ChangeRole(ctx context.Context, actor, tenantID, userID, role string) (entity.Membership, error) {
	if err := s.authz.PlatformOr(ctx, actor, "can_manage_members", tenantID, entity.ErrForbidden); err != nil {
		return entity.Membership{}, err
	}
	m, err := s.members.Get(ctx, tenantID, userID)
	if err != nil {
		return entity.Membership{}, err
	}
	if m.Role == role {
		return m, nil
	}
	assigned, err := assignMemberRole(ctx, s.roles, s.relations, s.roleRels, tenantID, userID, m.Affiliation, role, m.CustomRoleID)
	if err != nil {
		return entity.Membership{}, err
	}
	if err := keepOneInternalAdmin(ctx, s.members, m); err != nil {
		return entity.Membership{}, err
	}

	if err := s.members.UpdateRole(ctx, tenantID, userID, assigned.StoredRole, assigned.CustomRoleID); err != nil {
		return entity.Membership{}, err
	}
	err = s.guard.Grant(ctx, assigned.Writes, assigned.Deletes, func(ctx context.Context) error {
		return s.members.UpdateRole(ctx, tenantID, userID, m.Role, m.CustomRoleID)
	})
	if err != nil {
		return entity.Membership{}, err
	}
	return s.members.Get(ctx, tenantID, userID)
}
